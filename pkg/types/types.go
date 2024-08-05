package types

type CustomObject struct {
	Obj        interface{} `json:"obj"`
	Action     string      `json:"action"`
	UpdatedObj interface{} `json:"updatedObj,omitempty"`
}
