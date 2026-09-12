package room

import (
	"encoding/json"
	"fmt"
)

type Changes struct {
	Header
	Events []Change `json:"events"`
}

func (*Changes) eventType() string { return "changes" }
func NewChanges(events []Change) *Changes {
	out := make([]Change, 0, len(events))
	for _, ch := range events {
		ch.kind().Type = ch.changeType()
		out = append(out, ch)
	}
	return &Changes{Events: out}
}
func (c *Changes) UnmarshalJSON(b []byte) error {
	var raw struct {
		Header
		Events []json.RawMessage `json:"events"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	events := make([]Change, 0, len(raw.Events))
	for _, one := range raw.Events {
		ch, err := DecodeChange(one)
		if err != nil {
			return err
		}
		events = append(events, ch)
	}
	c.Header, c.Events = raw.Header, events
	return nil
}
func DecodeChange(b []byte) (Change, error) {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(b, &envelope); err != nil {
		return nil, err
	}
	entry, ok := changeTypes[envelope.Type]
	if !ok {
		return nil, fmt.Errorf("room: no event called %q", envelope.Type)
	}
	ch := entry.build()
	if err := json.Unmarshal(b, ch); err != nil {
		return nil, err
	}
	return ch, nil
}
