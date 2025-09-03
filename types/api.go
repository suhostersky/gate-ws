package types

import "encoding/json"

type ApiRequest struct {
	App     string     `json:"app,omitempty"`
	Time    int64      `json:"time"`
	Id      *int64     `json:"id,omitempty"`
	Channel string     `json:"channel"`
	Event   string     `json:"event,omitempty"`
	Payload ApiPayload `json:"payload,omitempty"`
}
type ApiPayload struct {
	ApiKey       string          `json:"api_key,omitempty"`
	Signature    string          `json:"signature,omitempty"`
	Timestamp    string          `json:"timestamp,omitempty"`
	RequestId    string          `json:"req_id,omitempty"`
	RequestParam json.RawMessage `json:"req_param,omitempty"`
}

type OrderParam struct {
	Contract   string `json:"contract"`
	Size       int64  `json:"size,omitempty"`
	Iceberg    int64  `json:"iceberg,omitempty"`
	Price      string `json:"price,omitempty"`
	Close      bool   `json:"close,omitempty"`
	ReduceOnly bool   `json:"reduce_only,omitempty"`
	Tif        string `json:"tif,omitempty"`
	Text       string `json:"text,omitempty"`
	AutoSize   string `json:"auto_size,omitempty"`
	StpAct     bool   `json:"stp_act,omitempty"`
}
