package main

import (
	"errors"
	"fmt"
	"github.com/tarantool/go-tarantool"
	"gopkg.in/vmihailenco/msgpack.v2"
	"math/rand"
	"reflect"
	"time"
)

const (
	HEX_LETTERS_DIGITS  = "abcdef0123456789"
	PUSH_TECH_GCM_DEBUG = "gd"
	AUTH_STRING_LEN     = 16
)

// Have to match push_db_server.lua
type ResultCode int

const (
	RES_OK ResultCode = 0

	RES_ERR_UNKNOWN_DEV_ID            ResultCode = -1
	RES_ERR_UNKNOWN_SUB_ID            ResultCode = -2
	RES_ERR_MISMATCHING_SUB_ID_DEV_ID ResultCode = -3

	RES_ERR_DATABASE ResultCode = -100
)

func (r *ResultCode) String() string {
	switch *r {
	case RES_OK:
		return "ok"
	case RES_ERR_UNKNOWN_DEV_ID:
		return "errUnknownDeviceId"
	case RES_ERR_UNKNOWN_SUB_ID:
		return "errUnknownSubId"
	case RES_ERR_MISMATCHING_SUB_ID_DEV_ID:
		return "errMismatchingSubIdDevId"
	case RES_ERR_DATABASE:
		return "errDatabase"
	default:
		return fmt.Sprintf("Unknown: %d", *r)
	}
}

/* ----- */

func genRandomString(keylen int) string {
	l := len(HEX_LETTERS_DIGITS)
	b := make([]byte, keylen)
	for i := 0; i < keylen; i++ {
		b[i] = HEX_LETTERS_DIGITS[rand.Intn(l)]
	}
	return string(b)
}

func genPushToken() string {
	l := len(HEX_LETTERS_DIGITS)
	b := make([]byte, 160)
	for i := 0; i < 40; i++ {
		v := HEX_LETTERS_DIGITS[rand.Intn(l)]
		b[i] = v
		b[i+40] = v
		b[i+80] = v
		b[i+120] = v
	}
	return string(b)
}

/* ----- */

type Millitime int64

const (
	TIME_MS_SECOND     Millitime = 1000
	TIME_MS_500_MILLIS Millitime = 500
	TIME_MS_1_SECOND   Millitime = TIME_MS_SECOND * 1
	TIME_MS_5_SECONDS  Millitime = TIME_MS_SECOND * 5
	TIME_MS_10_SECONDS Millitime = TIME_MS_SECOND * 10
	TIME_MS_15_SECONDS Millitime = TIME_MS_SECOND * 15
	TIME_MS_1_MINUTE   Millitime = TIME_MS_SECOND * 60 * 1
	TIME_MS_5_MINUTES  Millitime = TIME_MS_SECOND * 60 * 5
	TIME_MS_10_MINUTES Millitime = TIME_MS_SECOND * 60 * 10
	TIME_MS_15_MINUTES Millitime = TIME_MS_SECOND * 60 * 15
	TIME_MS_30_MINUTES Millitime = TIME_MS_SECOND * 60 * 30
	TIME_MS_1_HOUR     Millitime = TIME_MS_SECOND * 60 * 60
	TIME_MS_1_DAY      Millitime = TIME_MS_SECOND * 60 * 60 * 24
	TIME_MS_5_DAYS     Millitime = TIME_MS_SECOND * 60 * 60 * 24 * 5
)

func milliTime() Millitime {
	return Millitime(time.Now().UnixNano() / int64(time.Millisecond))
}

func milliTimeToTime(t Millitime) time.Time {
	return time.Unix(int64(t)/1000, (int64(t)%1000)*1000000)
}

func milliTimeToMillis(t Millitime) int64 {
	return int64(t)
}

func milliTimeFormat(t Millitime) string {
	tt := milliTimeToTime(t)
	return tt.Format("2006-01-02 15:04:05.000 MST")
}

func encodeMilliTime(e *msgpack.Encoder, t Millitime) error {
	return e.EncodeInt64(int64(t))
}

func decodeMilliTime(d *msgpack.Decoder) (t Millitime, err error) {
	i, err := d.DecodeInt64()
	t = Millitime(i)
	return
}

/* ----- */

type DevEnt struct {
	dev_id           string
	auth             string
	push_token       string
	push_tech        string
	ping_ts          Millitime
	change_ts        Millitime
	change_count     int
	change_priority  int
	send_error_count int
	not_reg_last_ts  Millitime
	not_reg_count    int
}

func (dev DevEnt) String() string {
	return fmt.Sprintf("[dev_id = %q, auth = %q, push_token = %q, push_tech = %q, "+
		"ping_ts = %s, change_ts = %s, change_priority = %t, "+
		"not_reg_last_ts = %s, not_reg_count = %d]",
		dev.dev_id, dev.auth, dev.push_token, dev.push_tech,
		milliTimeFormat(dev.ping_ts), milliTimeFormat(dev.change_ts), dev.change_priority != 0,
		milliTimeFormat(dev.not_reg_last_ts), dev.not_reg_count)
}

func encodeDevEnt(e *msgpack.Encoder, v reflect.Value) error {
	m := v.Interface().(DevEnt)
	if err := e.EncodeSliceLen(11); err != nil {
		return err
	}
	if err := e.EncodeString(m.dev_id); err != nil {
		return err
	}
	if err := e.EncodeString(m.auth); err != nil {
		return err
	}
	if err := e.EncodeString(m.push_token); err != nil {
		return err
	}
	if err := e.EncodeString(m.push_tech); err != nil {
		return err
	}
	if err := encodeMilliTime(e, m.ping_ts); err != nil {
		return err
	}
	if err := encodeMilliTime(e, m.change_ts); err != nil {
		return err
	}
	if err := e.EncodeInt(m.change_count); err != nil {
		return err
	}
	if err := e.EncodeInt(m.change_priority); err != nil {
		return err
	}
	if err := e.EncodeInt(m.send_error_count); err != nil {
		return err
	}
	if err := encodeMilliTime(e, m.not_reg_last_ts); err != nil {
		return err
	}
	if err := e.EncodeInt(m.not_reg_count); err != nil {
		return err
	}
	return nil
}

func decodeDevEnt(d *msgpack.Decoder, v reflect.Value) error {
	var err error
	var l int
	m := v.Addr().Interface().(*DevEnt)
	if l, err = d.DecodeSliceLen(); err != nil {
		return err
	}
	if l != 11 {
		return fmt.Errorf("decodeDevEnt array len doesn't match: %d", l)
	}
	if m.dev_id, err = d.DecodeString(); err != nil {
		return err
	}
	if m.auth, err = d.DecodeString(); err != nil {
		return err
	}
	if m.push_token, err = d.DecodeString(); err != nil {
		return err
	}
	if m.push_tech, err = d.DecodeString(); err != nil {
		return err
	}
	if m.ping_ts, err = decodeMilliTime(d); err != nil {
		return err
	}
	if m.change_ts, err = decodeMilliTime(d); err != nil {
		return err
	}
	if m.change_count, err = d.DecodeInt(); err != nil {
		return err
	}
	if m.change_priority, err = d.DecodeInt(); err != nil {
		return err
	}
	if m.send_error_count, err = d.DecodeInt(); err != nil {
		return err
	}
	if m.not_reg_last_ts, err = decodeMilliTime(d); err != nil {
		return err
	}
	if m.not_reg_count, err = d.DecodeInt(); err != nil {
		return err
	}
	return nil
}

type SubEnt struct {
	sub_id       string
	dev_id       string
	ping_ts      Millitime
	change_ts    Millitime
	folder_id    string
	ews_is_alive bool
	ews_is_dead  bool
}

func (sub SubEnt) String() string {
	return fmt.Sprintf("[sub_id = %q, dev_id = %q, ping_ts = %s, change_ts = %s, folder_id = %s, alive = %t, dead = %t]",
		sub.sub_id, sub.dev_id,
		milliTimeFormat(sub.ping_ts),
		milliTimeFormat(sub.change_ts),
		sub.folder_id,
		sub.ews_is_alive, sub.ews_is_dead)
}

func encodeSubEnt(e *msgpack.Encoder, v reflect.Value) error {
	m := v.Interface().(SubEnt)
	if err := e.EncodeSliceLen(7); err != nil {
		return err
	}
	if err := e.EncodeString(m.sub_id); err != nil {
		return err
	}
	if err := e.EncodeString(m.dev_id); err != nil {
		return err
	}
	if err := encodeMilliTime(e, m.ping_ts); err != nil {
		return err
	}
	if err := encodeMilliTime(e, m.change_ts); err != nil {
		return err
	}
	if err := e.EncodeString(m.folder_id); err != nil {
		return err
	}
	ews_is_alive := 0
	if m.ews_is_alive {
		ews_is_alive = 1
	}
	if err := e.EncodeInt(ews_is_alive); err != nil {
		return err
	}
	ews_is_dead := 0
	if m.ews_is_dead {
		ews_is_dead = 1
	}
	if err := e.EncodeInt(ews_is_dead); err != nil {
		return err
	}
	return nil
}

func decodeSubEnt(d *msgpack.Decoder, v reflect.Value) error {
	var err error
	var l int
	m := v.Addr().Interface().(*SubEnt)
	if l, err = d.DecodeSliceLen(); err != nil {
		return err
	}
	if l != 7 {
		return fmt.Errorf("decodeSubEnt array len doesn't match: %d", l)
	}
	if m.sub_id, err = d.DecodeString(); err != nil {
		return err
	}
	if m.dev_id, err = d.DecodeString(); err != nil {
		return err
	}
	if m.ping_ts, err = decodeMilliTime(d); err != nil {
		return err
	}
	if m.change_ts, err = decodeMilliTime(d); err != nil {
		return err
	}
	if m.folder_id, err = d.DecodeString(); err != nil {
		return err
	}
	var ews_is_alive int
	if ews_is_alive, err = d.DecodeInt(); err != nil {
		return err
	}
	m.ews_is_alive = ews_is_alive != 0
	var ews_is_dead int
	if ews_is_dead, err = d.DecodeInt(); err != nil {
		return err
	}
	m.ews_is_dead = ews_is_dead != 0
	return nil
}

type ResultEnt struct {
	code ResultCode
	s    string
}

func (res ResultEnt) String() string {
	return fmt.Sprintf("[code = %s, s = %q]",
		&res.code, res.s)
}

func encodeResultEnt(e *msgpack.Encoder, v reflect.Value) error {
	m := v.Interface().(ResultEnt)
	if err := e.EncodeSliceLen(2); err != nil {
		return err
	}
	if err := e.EncodeInt(int(m.code)); err != nil {
		return err
	}
	if err := e.EncodeString(m.s); err != nil {
		return err
	}
	return nil
}

func decodeResultEnt(d *msgpack.Decoder, v reflect.Value) error {
	var err error
	var l int
	m := v.Addr().Interface().(*ResultEnt)
	if l, err = d.DecodeSliceLen(); err != nil {
		return err
	}
	if l < 1 || l > 2 {
		return fmt.Errorf("decodeResultEnt array len doesn't match: %d", l)
	}
	if code, err := d.DecodeInt(); err != nil {
		return err
	} else {
		m.code = ResultCode(code)
	}
	if l >= 2 {
		if m.s, err = d.DecodeString(); err != nil {
			return err
		}
	} else {
		m.s = ""
	}
	return nil
}

type ResultSubListEnt struct {
	code ResultCode
	subs []SubEnt
}

func (res ResultSubListEnt) String() string {
	return fmt.Sprintf("[code = %s, subs = %d]",
		&res.code, len(res.subs))
}

func encodeResultSubListEnt(e *msgpack.Encoder, v reflect.Value) error {
	var mlen int
	m := v.Interface().(ResultSubListEnt)

	if m.subs == nil {
		mlen = 1
	} else {
		mlen = 2
	}

	if err := e.EncodeSliceLen(mlen); err != nil {
		return err
	}
	if err := e.EncodeInt(int(m.code)); err != nil {
		return err
	}

	if m.subs != nil {
		if err := e.EncodeSliceLen(len(m.subs)); err != nil {
			return err
		}
		for _, m := range m.subs {
			if err := e.Encode(m); err != nil {
				return err
			}
		}
	}
	return nil
}

func decodeResultSubListEnt(d *msgpack.Decoder, v reflect.Value) error {
	var err error
	var ml int
	m := v.Addr().Interface().(*ResultSubListEnt)
	if ml, err = d.DecodeSliceLen(); err != nil {
		return err
	}
	if ml < 1 || ml > 2 {
		return fmt.Errorf("decodeResultSubListEnt array len doesn't match: %d", ml)
	}
	if code, err := d.DecodeInt(); err != nil {
		return err
	} else {
		m.code = ResultCode(code)
	}
	if ml >= 2 {
		if ml, err = d.DecodeSliceLen(); err != nil {
			return err
		}
		m.subs = make([]SubEnt, ml)
		for i := 0; i < ml; i++ {
			if err := d.Decode(&m.subs[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func init() {
	msgpack.Register(reflect.TypeOf(DevEnt{}), encodeDevEnt, decodeDevEnt)
	msgpack.Register(reflect.TypeOf(SubEnt{}), encodeSubEnt, decodeSubEnt)
	msgpack.Register(reflect.TypeOf(ResultEnt{}), encodeResultEnt, decodeResultEnt)
	msgpack.Register(reflect.TypeOf(ResultSubListEnt{}), encodeResultSubListEnt, decodeResultSubListEnt)
}

/* ----- */

type PushDbModel struct {
	dbconn *tarantool.Connection
}

func NewPushDbModel(dbconn *tarantool.Connection) *PushDbModel {
	model := &PushDbModel{dbconn: dbconn}

	return model
}

func (model *PushDbModel) getDevEnt(dev_id string) (*DevEnt, error) {
	var res []DevEnt
	err := model.dbconn.SelectTyped("devs", "primary", 0, 1, tarantool.IterEq, []interface{}{dev_id}, &res)
	if err != nil {
		s := fmt.Sprintf("Error calling device select: %s", err)
		return nil, errors.New(s)
	}

	if res == nil {
		s := "Error calling device result set"
		return nil, errors.New(s)
	}

	if len(res) == 0 {
		return nil, nil
	}

	return &res[0], nil
}

func (model *PushDbModel) doCreateDev(dev_id string, auth string, push_token string, push_tech string, now Millitime) (*DevEnt, ResultCode, error) {
	var fname = "push_CreateDev"

	var res []DevEnt
	err := model.dbconn.CallTyped(fname, []interface{}{dev_id, auth, push_token, push_tech, now}, &res)
	if err != nil {
		s := fmt.Sprintf("Error calling %s: %s", fname, err.Error())
		return nil, RES_ERR_DATABASE, errors.New(s)
	}

	if res == nil || len(res) != 1 {
		s := fmt.Sprintf("Error calling %s: result set", fname)
		return nil, RES_ERR_DATABASE, errors.New(s)
	}

	return &res[0], RES_OK, nil
}

func (model *PushDbModel) doCreateSub(dev_id string, folder_id string, sub_id string, now Millitime) (ResultCode, error) {
	var fname = "push_CreateSub"

	var res []ResultEnt
	err := model.dbconn.CallTyped(fname, []interface{}{dev_id, folder_id, sub_id, now}, &res)
	if err != nil {
		s := fmt.Sprintf("Error calling %s: %s", fname, err.Error())
		return RES_ERR_DATABASE, errors.New(s)
	}
	if res == nil || len(res) != 1 {
		s := fmt.Sprintf("Error calling %s: result set", fname)
		return RES_ERR_DATABASE, errors.New(s)
	}

	return res[0].code, nil
}

func (model *PushDbModel) doPingSub(dev_id string, folder_id string, sub_id string, now Millitime) (ResultCode, error) {
	var fname = "push_PingSub"

	var res []ResultEnt

	err := model.dbconn.CallTyped(fname, []interface{}{dev_id, folder_id, sub_id, now}, &res)
	if err != nil {
		s := fmt.Sprintf("Error calling %s: %s", fname, err.Error())
		return RES_ERR_DATABASE, errors.New(s)
	}
	if res == nil || len(res) != 1 {
		s := fmt.Sprintf("Error calling %s: result set", fname)
		return RES_ERR_DATABASE, errors.New(s)
	}

	return res[0].code, nil
}

func (model *PushDbModel) doChangeSub(dev_id string, folder_id string, sub_id string, now Millitime, delta Millitime, priority bool) (ResultCode, error) {
	var fname = "push_ChangeSub"

	pint := 0
	if priority {
		pint = 1
	}

	var res []ResultEnt

	err := model.dbconn.CallTyped(fname, []interface{}{dev_id, folder_id, sub_id, now, delta, pint}, &res)
	if err != nil {
		s := fmt.Sprintf("Error calling %s: %s", fname, err.Error())
		return RES_ERR_DATABASE, errors.New(s)
	}
	if res == nil || len(res) != 1 {
		s := fmt.Sprintf("Error calling %s: result set", fname)
		return RES_ERR_DATABASE, errors.New(s)
	}

	return res[0].code, nil
}
