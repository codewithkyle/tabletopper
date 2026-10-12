package dnd

import "testing"

func TestEveryChallengeRatingHasItsNumbers(t *testing.T) {
	ratings := ChallengeRatings()
	if len(ratings) != 34 {
		t.Fatalf("%d ratings, want 34 (0, 1/8, 1/4, 1/2 and 1 through 30)", len(ratings))
	}
	if ratings[0].Value != "0" || ratings[1].Value != "1/8" || ratings[2].Value != "1/4" || ratings[3].Value != "1/2" {
		t.Errorf("the list does not open with the four ratings below CR 1: %v", ratings[:4])
	}
	if last := ratings[len(ratings)-1]; last.Value != "30" {
		t.Errorf("the list ends at %q, want 30", last.Value)
	}
	for i, rating := range ratings {
		if rating.Label == "" {
			t.Errorf("rating %q has no label", rating.Value)
		}
		if i > 0 && rating.XP <= ratings[i-1].XP {
			t.Errorf("CR %s is worth %d XP, which is not more than CR %s at %d",
				rating.Value, rating.XP, ratings[i-1].Value, ratings[i-1].XP)
		}
		want := uint8(2)
		if i >= 4 {
			want = uint8(2 + (i-4)/4)
		}
		if rating.Proficiency != want {
			t.Errorf("CR %s has a proficiency bonus of +%d, want +%d", rating.Value, rating.Proficiency, want)
		}
	}
	if got := ratings[len(ratings)-1].Proficiency; got != 9 {
		t.Errorf("CR 30 has a proficiency bonus of +%d, want +9", got)
	}
}
