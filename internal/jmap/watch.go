package jmap

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/email"

	"github.com/dsh2dsh/goimapnotify/internal/box"
	"github.com/dsh2dsh/goimapnotify/internal/config"
)

const (
	maxBackoff   = 5 * time.Minute
	pingInterval = 300 // seconds
)

type WatchMailboxes struct {
	boxes       []*box.Box
	retries     int
	startupSync bool

	client *client
	events chan<- *box.IDLE

	jmapBoxes   *mailboxes
	eventSource string
	lastEventId string
	retryDelay  time.Duration
	emailState  string

	pingInterval   int
	watchdogTicker *time.Ticker
	stopWatchdog   func()
}

func NewWatchMailboxes(boxes []*box.Box, events chan<- *box.IDLE,
) *WatchMailboxes {
	return (&WatchMailboxes{
		boxes:        boxes,
		events:       events,
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

		slog.Error("Initial connection failed, retrying in background",
			slog.String("account", self.accountConfig().Alias),
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
	self.printConnected()

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

func (self *WatchMailboxes) printConnected() {
	jmapAccount := self.client.Account()
	l := slog.With(
		slog.String("account", self.accountConfig().Alias),
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

	l := slog.With(slog.String("account", self.accountConfig().Alias))
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
	l := slog.With(slog.String("account", self.accountConfig().Alias))
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
	for self.reconnect(ctx) {
		if ctx.Err() != nil {
			break
		}

		self.syncOnStart(ctx)

		if ok := self.watch(ctx); !ok || ctx.Err() != nil {
			break
		}
		self.Close()
	}

	if ctx.Err() == nil {
		self.sendEvent(ctx, self.boxes[0], box.StopWatching)
	}
	self.Close()
}

func (self *WatchMailboxes) sendEvent(ctx context.Context, b *box.Box,
	reason box.EventType,
) {
	select {
	case <-ctx.Done():
	case self.events <- box.NewEvent(b, reason):
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

func (self *WatchMailboxes) Close() {
	self.client = nil
}

func (self *WatchMailboxes) syncOnStart(ctx context.Context) {
	if !self.startupSync {
		return
	}

	l := slog.With(slog.String("account", self.accountConfig().Alias))

	for _, m := range self.jmapBoxes.Watching() {
		b := m.Watching()
		l.Info(
			"issuing fake event for first time sync (skipping post-commands)",
			slog.String("mailbox", b.Mailbox))
		self.sendEvent(ctx, b, box.EventSync)
	}
	self.startupSync = false
}

func (self *WatchMailboxes) watch(ctx context.Context) bool {
	watchdog, stopWatchdog := context.WithCancel(ctx)
	self.stopWatchdog = stopWatchdog
	defer self.stopWatchdog()

	l := slog.With(slog.String("account", self.accountConfig().Alias))
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

		slog.Info("listen for events from event source",
			slog.String("account", self.accountConfig().Alias),
			slog.String("url", self.eventSource))
		self.readEvents(req.Context(), resp.Body, yield)
	}
}

func (self *WatchMailboxes) readEvents(ctx context.Context, r io.Reader,
	yield func(*jmap.StateChange, error) bool,
) {
	var wg sync.WaitGroup
	if self.pingInterval > 0 {
		defer self.startWatchdog(ctx, &wg)()
	}

	var b bytes.Buffer
	var eventType string
	var stateChange jmap.StateChange
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Bytes()

		// Handle blank line (dispatch event)
		if len(line) == 0 {
			if !self.dispatchEvent(eventType, &b, &stateChange, yield) {
				return
			}
			eventType = ""
			continue
		}

		// Handle comment line
		if line[0] == ':' {
			continue
		}

		// Process field line
		field, val, _ := bytes.Cut(line, []byte(":"))
		if len(val) == 0 {
			switch string(field) {
			case "event":
				eventType = ""
			case "data":
				b.WriteByte('\n')
			case "id":
				self.lastEventId = ""
			}
			continue
		}
		val = bytes.TrimSpace(val)

		switch string(field) {
		case "event":
			eventType = string(val)
		case "data":
			b.Write(val)
			b.WriteByte('\n')
		case "id":
			self.processIdField(val)
		case "retry":
			self.processRetryField(string(val))
		}
	}

	if err := scanner.Err(); err != nil {
		if !errors.Is(err, context.Canceled) {
			yield(nil, fmt.Errorf("read event: %w", err))
		}
	} else if b.Len() != 0 {
		yield(nil, fmt.Errorf("read event: %w", io.ErrUnexpectedEOF))
	}
}

func (self *WatchMailboxes) startWatchdog(ctx context.Context,
	wg *sync.WaitGroup,
) (deferFunc func()) {
	d := self.watchdogInterval()
	l := slog.With(slog.String("account", self.accountConfig().Alias))
	l.Info("start ping watchdog", slog.Duration("timeout", d))

	self.watchdogTicker = time.NewTicker(d)
	wg.Go(func() { self.watchdog(ctx) })

	return func() {
		self.stopWatchdog()
		l.Info("wait for watchdog goroutine to stop...")
		wg.Wait()
	}
}

func (self *WatchMailboxes) watchdogInterval() time.Duration {
	return time.Duration(self.pingInterval)*time.Second + time.Minute
}

func (self *WatchMailboxes) watchdog(ctx context.Context) {
	l := slog.With(slog.String("account", self.accountConfig().Alias))
	select {
	case <-ctx.Done():
		self.watchdogTicker.Stop()
		l.Info("ping watchdog stopped", slog.Any("reason", ctx.Err()))
	case <-self.watchdogTicker.C:
		self.stopWatchdog()
		l.Info("ping watchdog timed out, reconnect to event source")
	}
}

func (self *WatchMailboxes) dispatchEvent(eventType string, b *bytes.Buffer,
	stateChange *jmap.StateChange, yield func(*jmap.StateChange, error) bool,
) bool {
	if b.Len() == 0 {
		return true
	}
	defer b.Reset()

	switch eventType {
	case "ping":
		self.processPing(b.Bytes())
		return true
	case "state":
	default:
		return true
	}

	if err := json.Unmarshal(b.Bytes(), stateChange); err != nil {
		yield(nil, fmt.Errorf("invalid state event: %w", err))
		return false
	}

	if !yield(stateChange, nil) {
		return false
	}
	clear(stateChange.Changed)
	return true
}

func (self *WatchMailboxes) processPing(b []byte) {
	l := slog.With(slog.String("account", self.accountConfig().Alias))
	var ping struct {
		Interval uint32 `json:"interval"`
	}

	if err := json.Unmarshal(b, &ping); err != nil {
		l.Warn("invalid JMAP ping event", slog.Any("error", err))
		return
	}

	self.pingInterval = int(ping.Interval)
	timeout := self.watchdogInterval()
	l.Info("JMAP ping, reset watchdog",
		slog.Duration("interval", time.Duration(self.pingInterval)*time.Second),
		slog.Duration("timeout", timeout))
	self.watchdogTicker.Reset(timeout)
}

func (self *WatchMailboxes) processIdField(b []byte) {
	if b[0] != 0 {
		self.lastEventId = string(b)
	}
}

func (self *WatchMailboxes) processRetryField(val string) {
	ms, err := strconv.Atoi(val)
	if err != nil || ms <= 0 {
		return
	}

	self.retryDelay = time.Duration(ms) * time.Millisecond
	slog.Info("got retry event",
		slog.String("account", self.accountConfig().Alias),
		slog.Duration("retryDelay", self.retryDelay))
}

func (self *WatchMailboxes) stateChanged(ctx context.Context,
	stateChange *jmap.StateChange,
) {
	state := self.changedEmailState(stateChange)
	if state == "" {
		return
	}

	l := slog.With(slog.String("account", self.accountConfig().Alias))
	if self.emailState == "" || self.emailState == state {
		self.emailState = state
		l.Debug("got current Email state", slog.String("state", state))
		return
	}
	l.Debug("got Email state change", slog.String("state", state))

	for {
		hasMore, err := self.fetchEmailChanges(ctx)
		if err != nil {
			l.Error("unable fetch Email changes",
				slog.String("sinceState", self.emailState), slog.Any("error", err))
		}
		if !hasMore {
			break
		}
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
			Properties: []string{"mailboxIds", "threadId"},
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

	slog.Debug("got Email changes",
		slog.String("account", self.accountConfig().Alias),
		slog.String("oldState", r.OldState),
		slog.String("newState", r.NewState),
		slog.Any("created", r.Created),
		slog.Any("updated", r.Updated),
		slog.Any("destroyed", r.Destroyed),
		slog.Bool("hasMoreChanges", r.HasMoreChanges))

	notifiers := [...]func(context.Context, []*email.Email) error{
		self.notifyCreated,
		self.notifyUpdated,
	}

	for i, fn := range notifiers {
		r, ok := resp.Responses[i+1].Args.(*email.GetResponse)
		if !ok {
			return false, fmt.Errorf("unexpected jmap response[%d]: %T", i+1,
				resp.Responses[i+1].Args)
		} else if len(r.List) == 0 {
			continue
		}

		if err := fn(ctx, r.List); err != nil {
			return false, fmt.Errorf("process fetched Emails: %w", err)
		}
	}

	if len(r.Destroyed) != 0 {
		if err := self.notifyDeleted(ctx, r.Destroyed); err != nil {
			return false, fmt.Errorf("process destroyed Emails: %w", err)
		}
	}

	self.emailState = r.NewState
	return r.HasMoreChanges, nil
}

func (self *WatchMailboxes) notifyCreated(ctx context.Context,
	emails []*email.Email,
) error {
	mailboxes := make(map[jmap.ID]int)
	for _, m := range emails {
		self.jmapBoxes.AddEmail(m)
		for id := range m.MailboxIDs {
			mailboxes[id]++
		}
	}
	return self.syncMailboxes(ctx, mailboxes, box.EventNewMail)
}

func (self *WatchMailboxes) notifyUpdated(ctx context.Context,
	emails []*email.Email,
) error {
	deleted, updated := self.updatedMailboxes(emails)

	events := []struct {
		mailboxes map[jmap.ID]int
		event     box.EventType
	}{
		{deleted, box.EventDeletedMail},
		{updated, box.EventFlagChanged},
	}

	for _, e := range events {
		if err := self.syncMailboxes(ctx, e.mailboxes, e.event); err != nil {
			return err
		}
	}
	return nil
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

func (self *WatchMailboxes) notifyDeleted(ctx context.Context, ids []jmap.ID,
) error {
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
	return self.syncMailboxes(ctx, mailboxes, box.EventDeletedMail)
}

func (self *WatchMailboxes) syncMailboxes(ctx context.Context,
	mailboxes map[jmap.ID]int, event box.EventType,
) error {
	for id, count := range mailboxes {
		mb := self.jmapBoxes.Mailbox(id)
		if mb == nil {
			continue
		}

		b := mb.Watching()
		if b == nil {
			continue
		}

		slog.Info("send Email change",
			slog.String("account", self.accountConfig().Alias),
			slog.String("event", event.String()),
			slog.String("mailbox", mb.Path()),
			slog.Int("count", count))
		self.sendEvent(ctx, b, event)
	}
	return nil
}
