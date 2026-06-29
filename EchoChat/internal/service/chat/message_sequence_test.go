package chat

import "testing"

func TestResolveConversationRecoveryFloor(t *testing.T) {
	tests := []struct {
		name          string
		highWaterSeq  int64
		messageMaxSeq int64
		wantFloor     int64
		wantSource    string
	}{
		{
			name:          "empty",
			highWaterSeq:  0,
			messageMaxSeq: 0,
			wantFloor:     0,
			wantSource:    "empty",
		},
		{
			name:          "message table only",
			highWaterSeq:  0,
			messageMaxSeq: 18,
			wantFloor:     18,
			wantSource:    "message_table_only",
		},
		{
			name:          "high water only",
			highWaterSeq:  21,
			messageMaxSeq: 0,
			wantFloor:     21,
			wantSource:    "high_water_only",
		},
		{
			name:          "message table catchup",
			highWaterSeq:  35,
			messageMaxSeq: 39,
			wantFloor:     39,
			wantSource:    "message_table_catchup",
		},
		{
			name:          "high water ahead",
			highWaterSeq:  44,
			messageMaxSeq: 40,
			wantFloor:     44,
			wantSource:    "high_water_ahead",
		},
		{
			name:          "equal",
			highWaterSeq:  52,
			messageMaxSeq: 52,
			wantFloor:     52,
			wantSource:    "equal",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			floor, source := resolveConversationRecoveryFloor(tc.highWaterSeq, tc.messageMaxSeq)
			if floor != tc.wantFloor {
				t.Fatalf("floor = %d, want %d", floor, tc.wantFloor)
			}
			if source != tc.wantSource {
				t.Fatalf("source = %s, want %s", source, tc.wantSource)
			}
		})
	}
}
