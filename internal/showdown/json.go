package showdown

import "encoding/json"

// parseSearch decodes the |updatesearch| payload.
func parseSearch(b Base, payload string) SearchUpdated {
	ev := SearchUpdated{Base: b}
	var raw struct {
		Searching []string          `json:"searching"`
		Games     map[string]string `json:"games"`
	}
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return ev
	}
	ev.Searching = raw.Searching
	ev.Games = raw.Games
	return ev
}

// parseChallenges decodes the |updatechallenges| payload.
func parseChallenges(b Base, payload string) ChallengesUpdated {
	ev := ChallengesUpdated{Base: b}
	var raw struct {
		ChallengesFrom map[string]string `json:"challengesFrom"`
		ChallengeTo    *struct {
			To     string `json:"to"`
			Format string `json:"format"`
		} `json:"challengeTo"`
	}
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return ev
	}
	ev.ChallengesFrom = raw.ChallengesFrom
	if raw.ChallengeTo != nil {
		ev.ChallengeTo = &Challenge{To: raw.ChallengeTo.To, Format: raw.ChallengeTo.Format}
	}
	return ev
}
