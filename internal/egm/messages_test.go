package egm

import (
	"testing"
	"unicode"
)

func TestUserMessagesAreEnglishOnly(t *testing.T) {
	_, reason, _, err := resolveEngine(&Options{Engine: EngineAuto}, &Settings{})
	if err != nil {
		t.Fatal(err)
	}
	for name, msg := range map[string]string{"engine fallback reason": reason, "msgUnstaged": msgUnstaged} {
		if msg == "" {
			t.Errorf("%s is empty", name)
		}
		for _, r := range msg {
			if r > unicode.MaxASCII {
				t.Errorf("%s has non-ASCII %q: %s", name, r, msg)
				break
			}
		}
	}
}
