package gcm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/log4go"
	"github.com/jpillora/backoff"
	"github.com/mattn/go-xmpp"
	"github.com/pborman/uuid"
)

const (
	CCSAck         = "ack"
	CCSNack        = "nack"
	CCSControl     = "control"
	CCSReceipt     = "receipt"
	HighPriority   = "high"
	LowPriority    = "low"
	httpAddress    = "https://gcm-http.googleapis.com/gcm/send"
	xmppHost       = "fcm-xmpp.googleapis.com"
	xmppPort       = "5235"
	xmppAddress    = xmppHost + ":" + xmppPort
	ccsMinBackoff  = 1 * time.Second
	unackThreshold = 100
)

var (
	DefaultMinBackoff = 1 * time.Second
	DefaultMaxBackoff = 10 * time.Second
	retryableErrors   = map[string]bool{
		"Unavailable":            true,
		"SERVICE_UNAVAILABLE":    true,
		"InternalServerError":    true,
		"INTERNAL_SERVER_ ERROR": true,
	}
)

type HttpMessage struct {
	To                    string        `json:"to,omitempty"`
	RegistrationIds       []string      `json:"registration_ids,omitempty"`
	CollapseKey           string        `json:"collapse_key,omitempty"`
	Priority              string        `json:"priority,omitempty"`
	ContentAvailable      *bool         `json:"content_available,omitempty"`
	DelayWhileIdle        *bool         `json:"delay_while_idle,omitempty"`
	TimeToLive            *int          `json:"time_to_live,omitempty"`
	RestrictedPackageName string        `json:"restricted_package_name,omitempty"`
	DryRun                *bool         `json:"dry_run,omitempty"`
	Data                  Data          `json:"data,omitempty"`
	Notification          *Notification `json:"notification,omitempty"`
}

type XmppMessage struct {
	To                       string        `json:"to,omitempty"`
	Condition                string        `json:"condition,omitempty"`
	MessageId                string        `json:"message_id"`
	MessageType              string        `json:"message_type,omitempty"`
	CollapseKey              string        `json:"collapse_key,omitempty"`
	Priority                 string        `json:"priority,omitempty"`
	ContentAvailable         *bool         `json:"content_available,omitempty"`
	DelayWhileIdle           *bool         `json:"delay_while_idle"`
	TimeToLive               *int          `json:"time_to_live,omitempty"`
	DeliveryReceiptRequested *bool         `json:"delivery_receipt_requested,omitempty"`
	DryRun                   *bool         `json:"dry_run,omitempty"`
	Data                     Data          `json:"data,omitempty"`
	Notification             *Notification `json:"notification,omitempty"`
}

type HttpResponse struct {
	MulticastId  int      `json:"multicast_id,omitempty"`
	Success      uint     `json:"success,omitempty"`
	Failure      uint     `json:"failure,omitempty"`
	CanonicalIds uint     `json:"canonical_ids,omitempty"`
	Results      []Result `json:"results,omitempty"`
	MessageId    int      `json:"message_id,omitempty"`
	Error        string   `json:"error,omitempty"`
}

type Result struct {
	MessageId      string `json:"message_id,omitempty"`
	RegistrationId string `json:"registration_id,omitempty"`
	Error          string `json:"error,omitempty"`
}

type CcsMessage struct {
	From             string `json:"from, omitempty"`
	MessageId        string `json:"message_id, omitempty"`
	MessageType      string `json:"message_type, omitempty"`
	RegistrationId   string `json:"registration_id,omitempty"`
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
	Category         string `json:"category, omitempty"`
	Data             Data   `json:"data,omitempty"`
	ControlType      string `json:"control_type,omitempty"`
}

type multicastResultsState map[string]*Result

type Data map[string]interface{}

type Notification struct {
	Title        string `json:"title,omitempty"`
	Body         string `json:"body,omitempty"`
	Icon         string `json:"icon,omitempty"`
	Sound        string `json:"sound,omitempty"`
	Badge        string `json:"badge,omitempty"`
	Tag          string `json:"tag,omitempty"`
	Color        string `json:"color,omitempty"`
	ClickAction  string `json:"click_action,omitempty"`
	BodyLocKey   string `json:"body_loc_key,omitempty"`
	BodyLocArgs  string `json:"body_loc_args,omitempty"`
	TitleLocArgs string `json:"title_loc_args,omitempty"`
	TitleLocKey  string `json:"title_loc_key,omitempty"`
}

type MessageHandler struct {
	OnAck       func(cm *CcsMessage)
	OnNAck      func(cm *CcsMessage)
	OnReceipt   func(cm *CcsMessage)
	OnSendError func(cm *CcsMessage)
	OnMessage   func(cm *CcsMessage)
}

type httpClient interface {
	send(apiKey string, m HttpMessage) (*HttpResponse, error)
	getRetryAfter() string
}

type httpGcmClient struct {
	GcmURL     string
	HttpClient *http.Client
	retryAfter string
}

func (c *httpGcmClient) send(apiKey string, m HttpMessage) (*HttpResponse, error) {
	bs, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("error marshalling message>%v", err)
	}
	log4go.Global.Debug("sending", string(bs))
	req, err := http.NewRequest("POST", c.GcmURL, bytes.NewReader(bs))
	if err != nil {
		return nil, fmt.Errorf("error creating request>%v", err)
	}
	req.Header.Add(http.CanonicalHeaderKey("Content-Type"), "application/json")
	req.Header.Add(http.CanonicalHeaderKey("Authorization"), authHeader(apiKey))
	httpResp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request to HTTP connection server>%v", err)
	}
	gcmResp := &HttpResponse{}
	body, err := ioutil.ReadAll(httpResp.Body)
	defer httpResp.Body.Close()
	if err != nil {
		return gcmResp, fmt.Errorf("error reading http response body>%v", err)
	}
	log4go.Global.Debug("received body", string(body))
	err = json.Unmarshal(body, &gcmResp)
	if err != nil {
		return gcmResp, fmt.Errorf("error unmarshaling json from body: %v", err)
	}
	// TODO(silvano): this is assuming that the header contains seconds instead of a date, need to check
	c.retryAfter = httpResp.Header.Get(http.CanonicalHeaderKey("Retry-After"))
	return gcmResp, nil
}

func (c httpGcmClient) getRetryAfter() string {
	return c.retryAfter
}

type xmppClient interface {
	Listen(h MessageHandler) error
	Send(m XmppMessage) (int, error)
}

type XmppGcmClient struct {
	sync.RWMutex
	XmppClient xmpp.Client
	messages   struct {
		Lock sync.RWMutex
		Cond *sync.Cond
		m    map[string]*messageLogEntry
	}
	senderID string
	apiKey   string
	closed   bool
}

type messageLogEntry struct {
	body    *XmppMessage
	backoff *exponentialBackoff
}

func NewXmppGcmClient(senderID string, apiKey string) (*XmppGcmClient, error) {
	nc, err := xmpp.NewClient(xmppAddress, xmppUser(senderID), apiKey, true)
	if err != nil {
		return nil, err
	}

	xc := &XmppGcmClient{
		XmppClient: *nc,
		messages: struct {
			Lock sync.RWMutex
			Cond *sync.Cond
			m    map[string]*messageLogEntry
		}{
			m: make(map[string]*messageLogEntry),
		},
		senderID: senderID,
		apiKey:   apiKey,
		closed:   true,
	}
	xc.messages.Cond = sync.NewCond(&xc.messages.Lock)
	return xc, nil
}

func (c *XmppGcmClient) Close() {
	c.closed = true
	c.XmppClient.Close()
}

func (c *XmppGcmClient) Listen(h MessageHandler) error {
	c.closed = false

	for {
		stanza, err := c.XmppClient.Recv()
		if err != nil {
			log4go.Warn("error on Recv>%v", err)

			if c.closed {
				return nil
			}

			//reconnect
			for {
				nc, err := xmpp.NewClient(xmppAddress, xmppUser(c.senderID), c.apiKey, true)
				if err != nil {
					log4go.Warn("error connect :%v", err)
					time.Sleep(3 * time.Second)
					continue
				}
				c.XmppClient = *nc
				break
			}

			c.messages.Lock.Lock()
			c.messages.m = make(map[string]*messageLogEntry)

			c.messages.Cond.Broadcast()
			c.messages.Lock.Unlock()
			log4go.Info("reconnect success, cleared map")
			continue
		}

		v, ok := stanza.(xmpp.Chat)
		if !ok {
			continue
		}
		switch v.Type {
		case "":
			cm := &CcsMessage{}
			err = json.Unmarshal([]byte(v.Other[0]), cm)
			if err != nil {
				log4go.Global.Debug("Error unmarshaling ccs message: %v", err)
				continue
			}
			switch cm.MessageType {
			case CCSAck:
				c.messages.Lock.Lock()
				if _, ok := c.messages.m[cm.MessageId]; ok {
					if h.OnAck != nil {
						go h.OnAck(cm)
					}
					delete(c.messages.m, cm.MessageId)
					c.messages.Cond.Signal()
				}
				c.messages.Lock.Unlock()
			case CCSNack:
				if retryableErrors[cm.Error] {
					c.retryMessage(*cm, h)
				} else {
					c.messages.Lock.Lock()
					if _, ok := c.messages.m[cm.MessageId]; ok {
						if h.OnNAck != nil {
							go h.OnNAck(cm)
						}
						delete(c.messages.m, cm.MessageId)
						c.messages.Cond.Signal()
					}
					c.messages.Lock.Unlock()
				}
			default:
				log4go.Global.Debug("Unknown ccs message: %v", cm)
			}
		case "normal":
			cm := &CcsMessage{}
			err = json.Unmarshal([]byte(v.Other[0]), cm)
			if err != nil {
				log4go.Global.Debug("Error unmarshaling ccs message: %v", err)
				continue
			}
			switch cm.MessageType {
			case CCSControl:
				log4go.Global.Warn("control message! %v", cm)
			case CCSReceipt:
				log4go.Global.Debug("receipt! %v", cm)
				origMessageID := strings.TrimPrefix(cm.MessageId, "dr2:")
				ack := XmppMessage{To: cm.From, MessageId: origMessageID, MessageType: CCSAck}
				c.reply(ack)
				if h.OnReceipt != nil {
					go h.OnReceipt(cm)
				}
			default:
				ack := XmppMessage{To: cm.From, MessageId: cm.MessageId, MessageType: CCSAck}
				c.reply(ack)
				if h.OnMessage != nil {
					go h.OnMessage(cm)
				}
			}
		case "error":
			log4go.Global.Debug("error response %v", v)
		default:
			log4go.Global.Debug("unknown message type %v", v)
		}
	}
}

func (c *XmppGcmClient) reply(m XmppMessage) {
	payload, err := formatStanza(m)
	log4go.Global.Info("[RPLY][%s]", m.MessageId)
	if err == nil {
		c.XmppClient.SendOrg(payload)
	}
}

func (c *XmppGcmClient) Send(m XmppMessage) (string, int, error) {
	if m.MessageId == "" {
		m.MessageId = uuid.New()
	}
	c.messages.Lock.Lock()
	for len(c.messages.m) >= unackThreshold {
		log4go.Global.Info("[FCTR]")
		c.messages.Cond.Wait()
	}

	if _, ok := c.messages.m[m.MessageId]; !ok {
		b := newExponentialBackoff()
		if b.b.Min < ccsMinBackoff {
			b.setMin(ccsMinBackoff)
		}
		c.messages.m[m.MessageId] = &messageLogEntry{body: &m, backoff: b}
	}
	c.messages.Lock.Unlock()

	payload, err := formatStanza(m)
	if err != nil {
		return m.MessageId, 0, err
	}

	to := m.To
	if len(m.To) > 32 {
		to = to[:32]
	}

	log4go.Global.Debug("[%v]", payload)
	for {
		bytes, err := c.XmppClient.SendOrg(payload)
		if err == nil {
			log4go.Global.Info("[SND][to:%s][mid:%s][type:%s]", to, m.MessageId, m.MessageType)
			return m.MessageId, bytes, err
		} else {
			log4go.Global.Warn("[SND_ERR][to:%s][mid:%s][type:%s]", to, m.MessageId, m.MessageType)
			time.Sleep(3 * time.Second)
		}
	}
}

func (c *XmppGcmClient) retryMessage(cm CcsMessage, h MessageHandler) {
	c.messages.Lock.RLock()
	defer c.messages.Lock.RUnlock()

	if me, ok := c.messages.m[cm.MessageId]; ok {
		if me.backoff.sendAnother() {
			go func(m *messageLogEntry) {
				m.backoff.wait()
				c.Send(*m.body)
			}(me)
		} else {
			if h.OnSendError != nil {
				go h.OnSendError(&cm)
			}
		}
	}
}

type backoffProvider interface {
	sendAnother() bool
	setMin(min time.Duration)
	wait()
}

type exponentialBackoff struct {
	b            backoff.Backoff
	currentDelay time.Duration
}

func newExponentialBackoff() *exponentialBackoff {
	b := &backoff.Backoff{
		Min:    DefaultMinBackoff,
		Max:    DefaultMaxBackoff,
		Jitter: true,
	}
	return &exponentialBackoff{b: *b, currentDelay: b.Duration()}
}

func (eb exponentialBackoff) sendAnother() bool {
	return eb.currentDelay <= eb.b.Max
}

func (eb *exponentialBackoff) setMin(min time.Duration) {
	eb.b.Min = min
	if (eb.currentDelay) < min {
		eb.currentDelay = min
	}
}

func (eb exponentialBackoff) wait() {
	time.Sleep(eb.currentDelay)
	eb.currentDelay = eb.b.Duration()
}

func SendHttp(apiKey string, m HttpMessage) (*HttpResponse, error) {
	c := &httpGcmClient{httpAddress, &http.Client{}, "0"}
	b := newExponentialBackoff()
	return sendHttp(apiKey, m, c, b)
}

func sendHttp(apiKey string, m HttpMessage, c httpClient, b backoffProvider) (*HttpResponse, error) {
	gcmResp := &HttpResponse{}
	var multicastId int
	targets, err := messageTargetAsStringsArray(m)
	if err != nil {
		return gcmResp, fmt.Errorf("error extracting target from message: %v", err)
	}
	localTo := make([]string, len(targets))
	copy(localTo, targets)
	resultsState := &multicastResultsState{}
	for b.sendAnother() {
		gcmResp, err = c.send(apiKey, m)
		if err != nil {
			return gcmResp, fmt.Errorf("error sending request to GCM HTTP server: %v", err)
		}
		if len(gcmResp.Results) > 0 {
			doRetry, toRetry, err := checkResults(gcmResp.Results, localTo, *resultsState)
			multicastId = gcmResp.MulticastId
			if err != nil {
				return gcmResp, fmt.Errorf("error checking GCM results: %v", err)
			}
			if doRetry {
				retryAfter, err := time.ParseDuration(c.getRetryAfter())
				if err != nil {
					b.setMin(retryAfter)
				}
				localTo = make([]string, len(toRetry))
				copy(localTo, toRetry)
				if m.RegistrationIds != nil {
					m.RegistrationIds = toRetry
				}
				b.wait()
				continue
			} else {
				break
			}
		} else {
			break
		}
	}

	if len(targets) > 1 {
		gcmResp = buildRespForMulticast(targets, *resultsState, multicastId)
	}
	return gcmResp, nil
}

func buildRespForMulticast(to []string, mrs multicastResultsState, mid int) *HttpResponse {
	resp := &HttpResponse{}
	resp.MulticastId = mid
	resp.Results = make([]Result, len(to))
	for i, regId := range to {
		result, ok := mrs[regId]
		if !ok {
			continue
		}
		resp.Results[i] = *result
		if result.MessageId != "" {
			resp.Success++
		} else if result.Error != "" {
			resp.Failure++
		}
		if result.RegistrationId != "" {
			resp.CanonicalIds++
		}
	}
	return resp
}

func messageTargetAsStringsArray(m HttpMessage) ([]string, error) {
	if m.RegistrationIds != nil {
		return m.RegistrationIds, nil
	} else if m.To != "" {
		target := []string{m.To}
		return target, nil
	}
	target := []string{}
	return target, fmt.Errorf("can't find any valid target field in message.")
}

func checkResults(gcmResults []Result, recipients []string, resultsState multicastResultsState) (doRetry bool, toRetry []string, err error) {
	doRetry = false
	toRetry = []string{}
	for i := 0; i < len(gcmResults); i++ {
		result := gcmResults[i]
		regId := recipients[i]
		resultsState[regId] = &result
		if result.Error != "" {
			if retryableErrors[result.Error] {
				toRetry = append(toRetry, regId)
				if doRetry == false {
					doRetry = true
				}
			}
		}
	}
	return doRetry, toRetry, nil
}

func authHeader(apiKey string) string {
	return fmt.Sprintf("key=%v", apiKey)
}

func xmppUser(senderId string) string {
	return senderId + "@gcm.googleapis.com"
}

func formatStanza(m XmppMessage) (string, error) {
	stanza := `<message id=""><gcm xmlns="google:mobile:data">%v</gcm></message>`
	body, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("could not unmarshal body of xmpp message>%v", err)
	}
	bs := string(body)

	return fmt.Sprintf(stanza, bs), nil
}
