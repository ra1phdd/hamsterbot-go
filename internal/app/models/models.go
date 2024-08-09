package models

type Mute struct {
	StartMute string `json:"start_mute"`
	Duration  int64  `json:"duration"`
}

type Payments struct {
	Payments []Payment `json:"payments"`
}
type Payment struct {
	To     int64 `json:"to"`
	From   int64 `json:"from"`
	Amount int   `json:"amount"`
}
