package jmap

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/email"

	"github.com/dsh2dsh/goimapnotify/internal/config"
	"github.com/dsh2dsh/goimapnotify/internal/logging"
	"github.com/dsh2dsh/goimapnotify/internal/model"
	"github.com/dsh2dsh/goimapnotify/internal/runner"
)

const (
	maxBackoff   = 5 * time.Minute
	pingInterval = 300 // seconds
)

type WatchMailboxes struct {
	boxes       []*model.Box
	retries     int
	startupSync bool
	events      chan<- *model.IDLE
	runner      *runner.Runner

	client      *client
	jmapBoxes   *mailboxes
	eventSource string
	lastEventId string
	retryDelay  time.Duration

	pingInterval   int
	watchdogTicker *time.Ticker
	stopWatchdog   func()

	wg            sync.WaitGroup
	emailState    string
	lastState     atomic.Value
	wakeupFetcher chan struct{}
}

func NewWatchMailboxes(boxes []*model.Box, events chan<- *model.IDLE,
	runner *runner.Runner,
) *WatchMailboxes {
	return (&WatchMailboxes{
		boxes:        boxes,
		events:       events,
		runner:       runner,
		pingInterval: pingInterval,
	}).init()
}

func (self *WatchMailboxes) init() *WatchMailboxes {
	cfg := self.accountConfig()
	if cfg.Ping != nil && *cfg.Ping >= 0 {
		self.pingInterval = *cfg.Ping
	}
	return self
}

func (self *WatchMailboxes) WithStartupSync(v bool) *WatchMailboxes {
	self.startupSync = v
	return self
}

func (self *WatchMailboxes) Connect(ctx context.Context, retries int) error {
	self.retries = retries

	if err := self.connect(ctx); err != nil {
		if unableWatch(err) {
			return err
		}

		logging.FromContext(ctx).Error(
			"Initial connection failed, retrying in background",
			slog.Any("error", err))
		return nil
	}
	return nil
}

func (self *WatchMailboxes) connect(ctx context.Context) error {
	c, err := New(ctx, self.accountConfig(), self.retries)
	if err != nil {
		return err
	}

	self.client = c
	self.initEventSource()
	self.printConnected(ctx)

	if err := self.initMailboxes(ctx); err != nil {
		return err
	}
	return nil
}

func (self *WatchMailboxes) accountConfig() *config.NotifyConfig {
	return self.boxes[0].Account()
}

func (self *WatchMailboxes) initEventSource() {
	s := self.client.Session.EventSourceURL
	s = strings.Replace(s, "{types}", string(mail.EmailEvent), 1)
	s = strings.Replace(s, "{closeafter}", "no", 1)
	s = strings.Replace(s, "{ping}", strconv.Itoa(self.pingInterval), 1)
	self.eventSource = s
}

func (self *WatchMailboxes) printConnected(ctx context.Context) {
	jmapAccount := self.client.Account()
	l := logging.FromContext(ctx).With(
		slog.String("name", jmapAccount.Name),
		slog.Bool("personal", jmapAccount.IsPersonal),
		slog.Bool("readOnly", jmapAccount.IsReadOnly))

	capabilities := jmapAccount.Capabilities[mail.URI]
	if mailCaps, ok := capabilities.(*mail.Mail); ok {
		l = l.With(
			slog.Uint64("maxMailboxesPerEmail", mailCaps.MaxMailboxesPerEmail),
			slog.Uint64("maxMailboxDepth", mailCaps.MaxMailboxDepth),
			slog.Uint64("maxSizeMailboxName", mailCaps.MaxSizeMailboxName),
			slog.Uint64("maxSizeAttachmentsPerEmail", mailCaps.MaxSizeAttachmentsPerEmail),
			slog.Bool("mayCreateTopLevelMailbox", mailCaps.MayCreateTopLevelMailbox),
			slog.Any("emailQuerySortOptions", mailCaps.EmailQuerySortOptions))
	}

	l.Info("connected",
		slog.String("sessionEndpoint", self.client.SessionEndpoint),
		slog.String("api", self.client.Session.APIURL),
		slog.String("eventSource", self.eventSource))
}

func (self *WatchMailboxes) initMailboxes(ctx context.Context) error {
	jmapBoxes, err := self.client.Mailboxes(ctx)
	if err != nil {
		return fmt.Errorf("unable list mailboxes: %w", err)
	}
	self.jmapBoxes = jmapBoxes

	if jmapBoxes.Len() == 0 {
		return fmt.Errorf("%w: no mailboxes found on server", ErrUnableWatch)
	}

	l := logging.FromContext(ctx)
	l.Info("got list of all mailboxes", slog.Int("count", jmapBoxes.Len()))

	for _, b := range self.boxes {
		l := l.With(slog.String("mailbox", b.Mailbox))
		if m := jmapBoxes.Watch(b); m != nil {
			l.Info("Watching mailbox")
		} else {
			l.Warn("mailbox not found, skipped!")
		}
	}

	watching := jmapBoxes.Watching()
	if len(watching) == 0 {
		return fmt.Errorf("%w: nothing to watch, no configured mailboxes found",
			ErrUnableWatch)
	}
	return self.queryEmails(ctx)
}

func (self *WatchMailboxes) queryEmails(ctx context.Context) error {
	l := logging.FromContext(ctx)
	l.Debug("query all Emails from watched mailboxes")

	var emailsCount int
	var queryState string
	emailQuery := email.Query{
		Account: self.client.AccountId(),
		Filter:  self.jmapBoxes.QueryFilter(),
	}

	for {
		var req jmap.Request
		callId := req.Invoke(&emailQuery)

		req.Invoke(&email.Get{
			Account:    self.client.AccountId(),
			Properties: []string{"mailboxIds"},
			ReferenceIDs: &jmap.ResultReference{
				ResultOf: callId,
				Name:     emailQuery.Name(),
				Path:     "/ids",
			},
		})

		resp, err := self.client.Do(ctx, &req)
		if err != nil {
			return fmt.Errorf("query Emails: %w", err)
		}

		q, ok := resp.Responses[0].Args.(*email.QueryResponse)
		if !ok {
			return fmt.Errorf("unexpected jmap response[0]: %T",
				resp.Responses[0].Args)
		} else if len(q.IDs) == 0 {
			break
		}

		if queryState == "" {
			queryState = q.QueryState
		} else if q.QueryState != queryState {
			l.Debug("query state changed, restart quering Emails",
				slog.String("was", queryState), slog.String("new", q.QueryState))
			emailsCount = 0
			queryState = ""
			emailQuery.Position = 0
			emailQuery.Limit = 0
			self.jmapBoxes.ClearEmails()
			continue
		}

		r, ok := resp.Responses[1].Args.(*email.GetResponse)
		if !ok {
			return fmt.Errorf("unexpected jmap response[1]: %T",
				resp.Responses[1].Args)
		}

		emailsCount += self.jmapBoxes.AddEmails(r.List)
		l.Debug("got queried Emails from watched mailboxes",
			slog.Int("ids", len(q.IDs)),
			slog.Int("count", len(r.List)),
			slog.Uint64("limit", q.Limit),
			slog.String("state", r.State))
		self.emailState = r.State

		if len(q.IDs) != int(q.Limit) {
			break
		}

		emailQuery.Position += int64(len(q.IDs))
		emailQuery.Limit = q.Limit
		l.Debug("query more Emails",
			slog.Uint64("position", uint64(emailQuery.Position)),
			slog.Uint64("limit", emailQuery.Limit))
	}

	l.Info("cached all Emails from watched mailboxes",
		slog.Int("count", emailsCount),
		slog.String("state", self.emailState))
	return nil
}

func (self *WatchMailboxes) Watch(ctx context.Context) {
	self.wakeupFetcher = make(chan struct{}, 1)
	self.wg.Go(func() { self.fetcher(ctx) })

	for self.reconnect(ctx) {
		if ctx.Err() != nil {
			break
		}

		self.syncOnStart(ctx)

		if ok := self.watch(ctx); !ok || ctx.Err() != nil {
			break
		}
		self.client = nil
	}

	if ctx.Err() == nil {
		self.sendEvent(ctx, self.boxes[0], model.StopWatching)
	}
	self.close(ctx)
}

func (self *WatchMailboxes) close(ctx context.Context) {
	close(self.wakeupFetcher)

	logging.FromContext(ctx).Info("waiting fetcher goroutine to stop...")
	self.wg.Wait()
}

func (self *WatchMailboxes) sendEvent(ctx context.Context, b *model.Box,
	reason model.EventType,
) {
	select {
	case <-ctx.Done():
	case self.events <- model.NewEvent(b, reason):
	}
}

func (self *WatchMailboxes) reconnect(ctx context.Context) bool {
	if self.client != nil {
		return true
	}

	l := slog.With(slog.String("alias", self.accountConfig().Alias))
	backoff := time.Second

	for {
		if !timeAfter(ctx, backoff) {
			l.Info("Reconnection cancelled, shutting down")
			return false
		}

		if err := self.connect(ctx); err != nil {
			if unableWatch(err) {
				l.Error("Reconnection failed", slog.Any("error", err))
				return false
			}

			backoff = min(backoff*2, maxBackoff)
			l.Error("Reconnection failed",
				slog.Duration("retrying", backoff),
				slog.Any("error", err))
			continue
		}

		l.Info("Reconnected successfully",
			slog.String("eventSource", self.eventSource))
		return true
	}
}

func (self *WatchMailboxes) syncOnStart(ctx context.Context) {
	if !self.startupSync {
		return
	}

	l := logging.FromContext(ctx)

	for _, m := range self.jmapBoxes.Watching() {
		b := m.Watching()
		l.Info(
			"issuing fake event for first time sync (skipping post-commands)",
			slog.String("mailbox", b.Mailbox))
		self.sendEvent(ctx, b, model.EventSync)
	}
	self.startupSync = false
}

func (self *WatchMailboxes) watch(ctx context.Context) bool {
	watchdog, stopWatchdog := context.WithCancel(ctx)
	self.stopWatchdog = stopWatchdog
	defer self.stopWatchdog()

	l := logging.FromContext(ctx)

	for {
		req, err := http.NewRequestWithContext(watchdog, http.MethodGet,
			self.eventSource, nil)
		if err != nil {
			l.Error("unable build event source request",
				slog.String("url", self.eventSource),
				slog.Any("error", err))
			return false
		}

		for stateChange, err := range self.listen(req) {
			if err != nil {
				l.Error("unable read even source", slog.Any("error", err))
				return true
			} else if stateChange.Type != "StateChange" {
				continue
			}
			self.stateChanged(ctx, stateChange)
		}

		if ctx.Err() != nil {
			return false
		}

		if self.retryDelay == 0 {
			l.Info("end of event source, reconnect")
			continue
		}

		l.Info("end of event source, reconnect after delay",
			slog.Duration("retryDelay", self.retryDelay))
		if !timeAfter(ctx, self.retryDelay) {
			return false
		}
	}
}

func (self *WatchMailboxes) listen(req *http.Request,
) iter.Seq2[*jmap.StateChange, error] {
	return func(yield func(*jmap.StateChange, error) bool) {
		if self.lastEventId != "" {
			req.Header.Set("Last-Event-ID", self.lastEventId)
		}

		resp, err := self.client.HttpClient.Do(req)
		if err != nil {
			yield(nil, fmt.Errorf("do event source request: %w", err))
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			yield(nil, fmt.Errorf("invalid event source response: %w",
				NewHttpError(resp.StatusCode)))
			return
		}

		ctx := req.Context()
		logging.FromContext(ctx).Info("listen for events from event source",
			slog.String("url", self.eventSource))
		self.readEvents(ctx, resp.Body, yield)
	}
}

func (self *WatchMailboxes) stateChanged(ctx context.Context,
	stateChange *jmap.StateChange,
) {
	state := self.changedEmailState(stateChange)
	if state == "" {
		return
	}

	logging.FromContext(ctx).Debug("got Email state change",
		slog.String("state", state))
	self.lastState.Store(state)

	select {
	case self.wakeupFetcher <- struct{}{}:
	default:
	}
}

func (self *WatchMailboxes) changedEmailState(stateChange *jmap.StateChange,
) string {
	changed, ok := stateChange.Changed[self.client.AccountId()]
	if ok {
		return changed[string(mail.EmailEvent)]
	}
	return ""
}

func (self *WatchMailboxes) fetcher(ctx context.Context) {
	l := logging.FromContext(ctx)
	l.Info("start background Email fetcher")

fetcherLoop:
	for {
		select {
		case <-ctx.Done():
			l.Info("Email fetcher stopped", slog.Any("reason", ctx.Err()))
			return
		case _, ok := <-self.wakeupFetcher:
			if !ok {
				l.Info("Email fetcher stopped by signal")
				return
			}

			state := self.lastState.Load().(string)
			if self.emailState == state {
				l.Debug("fetcher reached last known state", slog.String("state", state))
				continue fetcherLoop
			}
		}

		for {
			hasMore, err := self.fetchEmailChanges(ctx)
			if err != nil {
				l.Error("unable fetch Email changes",
					slog.String("sinceState", self.emailState), slog.Any("error", err))
				continue fetcherLoop
			} else if !hasMore {
				continue fetcherLoop
			}
		}
	}
}

func (self *WatchMailboxes) fetchEmailChanges(ctx context.Context,
) (bool, error) {
	var req jmap.Request
	emailChanges := email.Changes{
		Account:    self.client.AccountId(),
		SinceState: self.emailState,
	}
	callId := req.Invoke(&emailChanges)

	for _, p := range [...]string{"/created", "/updated"} {
		req.Invoke(&email.Get{
			Account:    self.client.AccountId(),
			Properties: []string{"mailboxIds", "threadId", "from", "subject"},
			ReferenceIDs: &jmap.ResultReference{
				ResultOf: callId,
				Name:     emailChanges.Name(),
				Path:     p,
			},
		})
	}

	resp, err := self.client.Do(ctx, &req)
	if err != nil {
		return false, err
	}

	r, ok := resp.Responses[0].Args.(*email.ChangesResponse)
	if !ok {
		return false, fmt.Errorf("unexpected jmap response[0]: %T",
			resp.Responses[0].Args)
	}

	logging.FromContext(ctx).Debug("got Email changes",
		slog.String("oldState", r.OldState),
		slog.String("newState", r.NewState),
		slog.Any("created", r.Created),
		slog.Any("updated", r.Updated),
		slog.Any("destroyed", r.Destroyed),
		slog.Bool("hasMoreChanges", r.HasMoreChanges))

	notifiers := [...]func(context.Context, []*email.Email){
		self.notifyCreated,
		self.notifyUpdated,
	}

	for i, fn := range notifiers {
		r, ok := resp.Responses[i+1].Args.(*email.GetResponse)
		if !ok {
			return false, fmt.Errorf("unexpected jmap response[%d]: %T", i+1,
				resp.Responses[i+1].Args)
		}
		fn(ctx, r.List)
	}
	self.notifyDeleted(ctx, r.Destroyed)

	self.emailState = r.NewState
	return r.HasMoreChanges, nil
}

func (self *WatchMailboxes) notifyCreated(ctx context.Context,
	emails []*email.Email,
) {
	if len(emails) == 0 {
		return
	}

	mailboxes := make(map[jmap.ID]int)
	threads := make(map[jmap.ID]map[jmap.ID]model.Thread)
	for _, m := range emails {
		self.jmapBoxes.AddEmail(m)
		for id := range m.MailboxIDs {
			mailboxes[id]++

			mailboxThreads, ok := threads[id]
			if !ok {
				mailboxThreads = make(map[jmap.ID]model.Thread)
				threads[id] = mailboxThreads
			}

			t, ok := mailboxThreads[m.ThreadID]
			if !ok {
				t.From = make(map[string]string)
			}

			for _, f := range m.From {
				address := strings.TrimSpace(f.Email)
				t.From[address] = strings.TrimSpace(f.Name)
			}
			t.Subject = m.Subject
			t.Count++
			mailboxThreads[m.ThreadID] = t
		}
	}
	self.syncMailboxes(ctx, mailboxes, model.EventNewMail)
	self.notifyNewMails(ctx, threads)
}

func (self *WatchMailboxes) notifyUpdated(ctx context.Context,
	emails []*email.Email,
) {
	deleted, updated := self.updatedMailboxes(emails)

	events := []struct {
		mailboxes map[jmap.ID]int
		event     model.EventType
	}{
		{deleted, model.EventDeletedMail},
		{updated, model.EventFlagChanged},
	}

	for _, e := range events {
		self.syncMailboxes(ctx, e.mailboxes, e.event)
	}
}

func (self *WatchMailboxes) updatedMailboxes(emails []*email.Email) (deleted,
	updated map[jmap.ID]int,
) {
	deleted = make(map[jmap.ID]int)
	updated = make(map[jmap.ID]int)

	for _, m := range emails {
		cached := self.jmapBoxes.Email(m.ID)
		if cached == nil {
			self.jmapBoxes.AddEmail(m)
			for id := range m.MailboxIDs {
				updated[id]++
			}
			continue
		}

		for id := range cached.MailboxIDs {
			if !m.MailboxIDs[id] {
				deleted[id]++
			}
		}

		for id := range m.MailboxIDs {
			updated[id]++
		}
		self.jmapBoxes.UpdateEmail(m)
	}
	return deleted, updated
}

func (self *WatchMailboxes) notifyDeleted(ctx context.Context, ids []jmap.ID) {
	if len(ids) == 0 {
		return
	}

	mailboxes := make(map[jmap.ID]int)
	for _, id := range ids {
		m := self.jmapBoxes.Email(id)
		if m == nil {
			continue
		}
		for id := range m.MailboxIDs {
			mailboxes[id]++
		}
		self.jmapBoxes.DeleteEmail(m.ID)
	}
	self.syncMailboxes(ctx, mailboxes, model.EventDeletedMail)
}

func (self *WatchMailboxes) syncMailboxes(ctx context.Context,
	mailboxes map[jmap.ID]int, event model.EventType,
) {
	l := logging.FromContext(ctx)
	for id, count := range mailboxes {
		mb := self.jmapBoxes.Mailbox(id)
		if mb == nil {
			continue
		}

		b := mb.Watching()
		if b == nil {
			continue
		}

		l.Info("send Email change",
			slog.String("event", event.String()),
			slog.String("mailbox", mb.Path()),
			slog.Int("count", count))
		self.sendEvent(ctx, b, event)
	}
}

func (self *WatchMailboxes) notifyNewMails(ctx context.Context,
	threads map[jmap.ID]map[jmap.ID]model.Thread,
) {
	for id, t := range threads {
		mb := self.jmapBoxes.Mailbox(id)
		if mb == nil {
			continue
		}

		b := mb.Watching()
		if b == nil || !b.NotifyNewMail {
			continue
		}

		l := logging.FromContext(ctx).With(slog.String("mailbox", mb.Path()))
		ctx := logging.WithLogger(ctx, l)
		mailboxThreads := slices.AppendSeq(make([]model.Thread, 0, len(t)),
			maps.Values(t))

		err := self.runner.NotifyNewMails(ctx, b, mailboxThreads)
		if err != nil {
			l.Error("unable notify new mail", slog.Any("error", err))
			return
		}
	}
}
