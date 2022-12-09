package game

// GameState the state of the game
type GameState int

const (
	k_Ready GameState = iota // game ready
	k_Game                   // gaming
	k_Over                   // game over
	k_Stop                   // game finish
)

const (
	MaxReadyTime int64  = 20
	MaxGameFrame uint32 = 30*60*3 + 100
)
