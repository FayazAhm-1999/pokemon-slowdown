package showdown

// Base carries the room a protocol event belongs to. An empty RoomID means the
// message was global rather than scoped to a room.
type Base struct {
	RoomID string
}

// Room returns the room the event belongs to ("" for global messages).
func (b Base) Room() string { return b.RoomID }

// Event is a typed server message. The UI and battle reducer consume these
// instead of raw protocol strings, so no component below this package ever
// needs to split a line like "|move|p1a: Dragapult|Draco Meteor|p2a: Garchomp".
type Event interface {
	Room() string
}

// ---------------------------------------------------------------------------
// Global messages
// ---------------------------------------------------------------------------

// ChallStr is the login challenge issued on connect.
type ChallStr struct {
	Base
	Challstr string
}

// UpdateUser reports a successful rename/login.
type UpdateUser struct {
	Base
	Name     string
	Named    bool
	Avatar   string
	Settings string
}

// FormatsUpdated replaces the server's format catalogue.
type FormatsUpdated struct {
	Base
	Formats []Format
}

// SearchUpdated reports current ladder searches and open games.
type SearchUpdated struct {
	Base
	Searching []string
	Games     map[string]string
}

// ChallengesUpdated reports incoming and outgoing challenges.
type ChallengesUpdated struct {
	Base
	ChallengesFrom map[string]string
	ChallengeTo    *Challenge
}

// QueryResponse carries the payload of a /query command.
type QueryResponse struct {
	Base
	Type string
	Data string
}

// Popup is a modal message from the server.
type Popup struct {
	Base
	Message string
}

// PM is a private message.
type PM struct {
	Base
	Sender   string
	Receiver string
	Message  string
}

// UserCount reports the number of users online.
type UserCount struct {
	Base
	Count int
}

// NameTaken reports a failed rename.
type NameTaken struct {
	Base
	Name    string
	Message string
}

// Notify is a server-sent notification.
type Notify struct {
	Base
	Title     string
	Message   string
	Highlight string
}

// ---------------------------------------------------------------------------
// Room messages
// ---------------------------------------------------------------------------

// RoomInit is the first message received when joining a room.
type RoomInit struct {
	Base
	Type string
}

// RoomTitle sets a room's display title.
type RoomTitle struct {
	Base
	Title string
}

// RoomUsers lists the users present in a chat room.
type RoomUsers struct {
	Base
	Users []User
}

// RoomMessage is plain, non-protocol text to display in a room log.
type RoomMessage struct {
	Base
	Message string
}

// RoomHTML is an HTML message. It is sanitised before display and never
// rendered as markup.
type RoomHTML struct {
	Base
	HTML string
}

// RoomJoin reports a user joining.
type RoomJoin struct {
	Base
	User string
}

// RoomLeave reports a user leaving.
type RoomLeave struct {
	Base
	User string
}

// RoomName reports a user renaming.
type RoomName struct {
	Base
	User  string
	OldID string
}

// ChatMessage is a chat line.
type ChatMessage struct {
	Base
	Time    int64
	User    string
	Message string
	Me      bool
}

// BattleStarted announces a battle between two users.
type BattleStarted struct {
	Base
	P1 string
	P2 string
}

// ---------------------------------------------------------------------------
// Battle initialisation
// ---------------------------------------------------------------------------

// BattlePlayer describes one side of a battle.
type BattlePlayer struct {
	Base
	Player string
	Name   string
	Avatar string
	Rating string
}

// BattleTeamSize reports how many Pokémon a side brought.
type BattleTeamSize struct {
	Base
	Player string
	Size   int
}

// BattleGameType is singles/doubles/triples/multi/freeforall.
type BattleGameType struct {
	Base
	GameType string
}

// BattleGen is the generation number.
type BattleGen struct {
	Base
	Gen int
}

// BattleTier is the format being played.
type BattleTier struct {
	Base
	Tier string
}

// BattleRated marks a battle as rated, with an optional message.
type BattleRated struct {
	Base
	Message string
}

// BattleRule is one rule line from the battle header.
type BattleRule struct {
	Base
	Rule string
}

// BattleClearpoke marks the start of team preview.
type BattleClearpoke struct{ Base }

// BattlePoke declares a Pokémon during team preview.
type BattlePoke struct {
	Base
	Player  string
	Details string
	Item    string
}

// BattleTeamPreview marks the team preview request.
type BattleTeamPreview struct{ Base }

// BattleStart marks the start of the battle proper.
type BattleStart struct{ Base }

// ---------------------------------------------------------------------------
// Battle progress
// ---------------------------------------------------------------------------

// BattleTurn advances the turn counter.
type BattleTurn struct {
	Base
	Turn int
}

// BattleRequest carries a raw JSON choice request.
type BattleRequest struct {
	Base
	Request string
}

// BattleWin ends the battle with a winner.
type BattleWin struct {
	Base
	User string
}

// BattleTie ends the battle in a draw.
type BattleTie struct{ Base }

// BattleInactive reports the battle timer turning on or off.
type BattleInactive struct {
	Base
	Message string
	On      bool
}

// BattleUpkeep marks the field-condition upkeep phase.
type BattleUpkeep struct{ Base }

// BattleTimestamp carries the server clock.
type BattleTimestamp struct {
	Base
	TS int64
}

// BattleError reports an invalid or unavailable choice.
type BattleError struct {
	Base
	Message string
}

// BattleClear clears the message bar and inserts a spacer.
type BattleClear struct{ Base }

// BattleMessage is a miscellaneous message from the simulator.
type BattleMessage struct {
	Base
	Message string
}

// ---------------------------------------------------------------------------
// Battle actions
// ---------------------------------------------------------------------------

// BattleMove reports a Pokémon using a move.
type BattleMove struct {
	Base
	User   string
	Move   string
	Target string
	Tags   Tags
}

// BattleSwitch reports an intentional or forced switch.
type BattleSwitch struct {
	Base
	User    string
	Details string
	HP      string
	Status  string
	Drag    bool
	Tags    Tags
}

// BattleDetailsChange reports a permanent forme change (e.g. Mega Evolution).
type BattleDetailsChange struct {
	Base
	User    string
	Details string
	HP      string
	Status  string
}

// BattleFormeChange reports a temporary forme change.
type BattleFormeChange struct {
	Base
	User    string
	Species string
	HP      string
	Status  string
}

// BattleReplace reports Illusion ending.
type BattleReplace struct {
	Base
	User    string
	Details string
	HP      string
	Status  string
}

// BattleSwap repositions an active Pokémon.
type BattleSwap struct {
	Base
	User     string
	Position int
}

// BattleCant reports a Pokémon being unable to act.
type BattleCant struct {
	Base
	User   string
	Reason string
	Move   string
}

// BattleFaint reports a fainted Pokémon.
type BattleFaint struct {
	Base
	User string
}

// BattleDamage reports a Pokémon losing HP.
type BattleDamage struct {
	Base
	Target string
	HP     string
	Status string
	Tags   Tags
}

// BattleHeal reports a Pokémon regaining HP.
type BattleHeal struct {
	Base
	Target string
	HP     string
	Status string
}

// BattleSetHP sets a Pokémon's HP directly.
type BattleSetHP struct {
	Base
	Target string
	HP     string
}

// BattleStatus reports a status condition being inflicted.
type BattleStatus struct {
	Base
	Target string
	Status string
}

// BattleCureStatus reports a status condition being cured.
type BattleCureStatus struct {
	Base
	Target string
	Status string
}

// BattleCureTeam reports a team-wide status cure.
type BattleCureTeam struct {
	Base
	User string
}

// BattleBoost reports a stat change.
type BattleBoost struct {
	Base
	Target string
	Stat   string
	Amount int
	Set    bool
	Tags   Tags
}

// BattleSwapBoost reports a stat swap.
type BattleSwapBoost struct {
	Base
	Source string
	Target string
	Stats  []string
}

// BattleInvertBoost reports inverted stat changes.
type BattleInvertBoost struct {
	Base
	Target string
}

// BattleClearBoost reports cleared stat changes.
type BattleClearBoost struct {
	Base
	Target string
}

// BattleClearAllBoost reports Haze-like global boost clearing.
type BattleClearAllBoost struct{ Base }

// BattleCopyBoost reports copied stat changes.
type BattleCopyBoost struct {
	Base
	Source string
	Target string
}

// BattleWeather reports the current weather.
type BattleWeather struct {
	Base
	Weather string
	Upkeep  bool
}

// BattleFieldStart reports a field condition starting.
type BattleFieldStart struct {
	Base
	Condition string
}

// BattleFieldEnd reports a field condition ending.
type BattleFieldEnd struct {
	Base
	Condition string
}

// BattleFieldActivate reports a one-shot field effect.
type BattleFieldActivate struct {
	Base
	Condition string
}

// BattleSideStart reports a side condition starting.
type BattleSideStart struct {
	Base
	Side      string
	Condition string
}

// BattleSideEnd reports a side condition ending.
type BattleSideEnd struct {
	Base
	Side      string
	Condition string
}

// BattleSwapSideConditions reports Court Change.
type BattleSwapSideConditions struct{ Base }

// BattleVolatileStart reports a volatile condition starting.
type BattleVolatileStart struct {
	Base
	Target string
	Effect string
}

// BattleVolatileEnd reports a volatile condition ending.
type BattleVolatileEnd struct {
	Base
	Target string
	Effect string
}

// BattleItem reports a revealed or changed item.
type BattleItem struct {
	Base
	Target string
	Item   string
	From   string
}

// BattleEndItem reports a consumed or destroyed item.
type BattleEndItem struct {
	Base
	Target string
	Item   string
	From   string
	Eat    bool
}

// BattleAbility reports a revealed or changed ability.
type BattleAbility struct {
	Base
	Target  string
	Ability string
	From    string
}

// BattleEndAbility reports a suppressed ability.
type BattleEndAbility struct {
	Base
	Target string
}

// BattleTransform reports Transform.
type BattleTransform struct {
	Base
	Target  string
	Species string
}

// BattleMega reports Mega Evolution.
type BattleMega struct {
	Base
	User  string
	Stone string
}

// BattlePrimal reports Primal Reversion.
type BattlePrimal struct {
	Base
	User string
}

// BattleBurst reports Ultra Burst.
type BattleBurst struct {
	Base
	User    string
	Species string
	Item    string
}

// BattleZPower reports a Z-Move powering up.
type BattleZPower struct {
	Base
	User string
}

// BattleZBroken reports a Z-Move breaking through Protect.
type BattleZBroken struct {
	Base
	Target string
}

// BattleHint is a parenthesised explanatory hint.
type BattleHint struct {
	Base
	Message string
}

// BattleEffect is any minor action without a dedicated type above. Kind is the
// protocol type without the leading '-' (e.g. "crit", "supereffective"), and
// Args holds the remaining fields. Nothing is discarded: unknown protocol
// events are preserved here rather than crashing the client.
type BattleEffect struct {
	Base
	Kind string
	Args []string
	Tags Tags
}

// BattleUnknown is a protocol line this client does not model at all. It is
// retained so debug logging and the log overlay can surface protocol drift.
type BattleUnknown struct {
	Base
	Type string
	Args []string
	Tags Tags
}
