package game

import "testing"

func TestTotalUnitPower(t *testing.T) {
	tests := []struct {
		name      string
		unitType  string
		count     int
		wantPower int
	}{
		{
			name:      "unknown unit type returns 0",
			unitType:  "dragon",
			count:     10,
			wantPower: 0,
		},
		{
			name:      "zero count returns 0",
			unitType:  UnitRecruit,
			count:     0,
			wantPower: 0,
		},
		{
			name:      "recruit power 1 per unit",
			unitType:  UnitRecruit,
			count:     50,
			wantPower: 50,
		},
		{
			name:      "knight power 3 per unit",
			unitType:  UnitKnight,
			count:     10,
			wantPower: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := totalUnitPower(tt.unitType, tt.count)
			if got != tt.wantPower {
				t.Errorf("totalUnitPower(%q, %d) = %d, want %d", tt.unitType, tt.count, got, tt.wantPower)
			}
		})
	}
}

func TestComputeLossRatios(t *testing.T) {
	tests := []struct {
		name        string
		atkPow      int
		defPow      int
		wantAtkLoss float64
		wantDefLoss float64
	}{
		{
			name:        "zero attacker power — no losses",
			atkPow:      0,
			defPow:      100,
			wantAtkLoss: 0,
			wantDefLoss: 0,
		},
		{
			name:        "zero defender power — no losses",
			atkPow:      100,
			defPow:      0,
			wantAtkLoss: 0,
			wantDefLoss: 0,
		},
		{
			name:        "both zero — no losses",
			atkPow:      0,
			defPow:      0,
			wantAtkLoss: 0,
			wantDefLoss: 0,
		},
		{
			name:        "equal power — both lose 30%",
			atkPow:      100,
			defPow:      100,
			wantAtkLoss: 0.30,
			wantDefLoss: 0.30,
		},
		{
			name: "attacker twice as strong — defender loses 30%, attacker loses 15%",
			// atkLoss = defPow/atkPow*0.3 = 100/200*0.3 = 0.15
			// defLoss = atkPow/defPow*0.3 = 200/100*0.3 = 0.60
			atkPow:      200,
			defPow:      100,
			wantAtkLoss: 0.15,
			wantDefLoss: 0.60,
		},
		{
			name: "overwhelming attacker — defender loss capped at 0.9",
			// defLoss = 1000/100*0.3 = 3.0 → capped at 0.9
			// atkLoss = 100/1000*0.3 = 0.03
			atkPow:      1000,
			defPow:      100,
			wantAtkLoss: 0.03,
			wantDefLoss: 0.90,
		},
		{
			name:        "overwhelming defender — attacker loss capped at 0.9",
			atkPow:      100,
			defPow:      1000,
			wantAtkLoss: 0.90,
			wantDefLoss: 0.03,
		},
	}

	const epsilon = 1e-9
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			atkLoss, defLoss := computeLossRatios(tt.atkPow, tt.defPow)
			if diff := atkLoss - tt.wantAtkLoss; diff > epsilon || diff < -epsilon {
				t.Errorf("atkLoss = %v, want %v", atkLoss, tt.wantAtkLoss)
			}
			if diff := defLoss - tt.wantDefLoss; diff > epsilon || diff < -epsilon {
				t.Errorf("defLoss = %v, want %v", defLoss, tt.wantDefLoss)
			}
		})
	}
}
