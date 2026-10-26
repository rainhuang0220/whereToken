package hosted

import "testing"
import "time"

func TestPublishBackoffIsImmediateThenHourly(t *testing.T) {
	if publishBackoff(0) != 0 || publishBackoff(1) != 0 {
		t.Fatal("first attempt must not wait")
	}
	if publishBackoff(2) != time.Hour || publishBackoff(9) != time.Hour {
		t.Fatal("later attempts wait one hour and are not discarded")
	}
}
