package postgres

import (
	"testing"
	"time"
)

func mustDate(t *testing.T, value string) time.Time {
	t.Helper()

	date, err := time.ParseInLocation(time.DateOnly, value, time.Local)
	if err != nil {
		t.Fatal(err)
	}
	return date
}

func TestAddCalendarMonthsForUgenBillingCases(t *testing.T) {
	tests := []struct {
		name   string
		start  string
		months int32
		want   string
	}{
		{
			name:   "monthly purchase on june nineteenth",
			start:  "2026-06-19",
			months: 1,
			want:   "2026-07-19",
		},
		{
			name:   "late recovery starts from payment date",
			start:  "2026-07-21",
			months: 1,
			want:   "2026-08-21",
		},
		{
			name:   "six month period",
			start:  "2026-06-19",
			months: 6,
			want:   "2026-12-19",
		},
		{
			name:   "annual period",
			start:  "2026-06-19",
			months: 12,
			want:   "2027-06-19",
		},
		{
			name:   "month end clamps to target month last day",
			start:  "2026-01-31",
			months: 1,
			want:   "2026-02-28",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := addCalendarMonths(mustDate(t, tt.start), tt.months).Format(time.DateOnly)
			if got != tt.want {
				t.Fatalf("addCalendarMonths(%s, %d) = %s, want %s", tt.start, tt.months, got, tt.want)
			}
		})
	}
}

func TestUgenPeriodCharge(t *testing.T) {
	tests := []struct {
		name   string
		price  float64
		period *ugenBillingPeriod
		want   float64
	}{
		{
			name:  "monthly has no discount",
			price: 63,
			period: &ugenBillingPeriod{
				months:          1,
				discountPercent: 0,
			},
			want: 63,
		},
		{
			name:  "six month discount",
			price: 63,
			period: &ugenBillingPeriod{
				months:          6,
				discountPercent: 13,
			},
			want: 328.86,
		},
		{
			name:  "annual discount",
			price: 98,
			period: &ugenBillingPeriod{
				months:          12,
				discountPercent: 24,
			},
			want: 893.76,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ugenPeriodCharge(tt.price, 1, tt.period)
			if got != tt.want {
				t.Fatalf("ugenPeriodCharge() = %.2f, want %.2f", got, tt.want)
			}
		})
	}
}

func TestUgenProratedUpgradeCharge(t *testing.T) {
	got := ugenProratedUpgradeCharge(
		63,
		98,
		1,
		1,
		0,
		mustDate(t, "2026-06-19"),
		mustDate(t, "2026-07-19"),
		mustDate(t, "2026-07-10"),
	)

	if got != 10.5 {
		t.Fatalf("ugenProratedUpgradeCharge() = %.2f, want 10.50", got)
	}
}

func TestDetermineAttachOpCancelDowngrade(t *testing.T) {
	cur := &attachedSubscription{
		fareId:        "pro",
		status:        "pending_downgrade",
		pendingFareId: "basic",
		farePrice:     98,
		subType:       "paid",
	}

	got := determineAttachOp(cur, "pro", 98)
	if got != fareOpCancelDowngrade {
		t.Fatalf("determineAttachOp() = %s, want %s", got, fareOpCancelDowngrade)
	}
}

func TestDetermineAttachOpSamePlanRenewal(t *testing.T) {
	cur := &attachedSubscription{
		fareId:    "pro",
		status:    "active",
		farePrice: 98,
		subType:   "paid",
	}

	got := determineAttachOp(cur, "pro", 98)
	if got != fareOpRenewal {
		t.Fatalf("determineAttachOp() = %s, want %s", got, fareOpRenewal)
	}
}

func TestDefaultUgenFreePeriod(t *testing.T) {
	period := defaultUgenFreePeriod()

	if period.code != "monthly" {
		t.Fatalf("period.code = %s, want monthly", period.code)
	}
	if period.months != 1 {
		t.Fatalf("period.months = %d, want 1", period.months)
	}
	if period.discountPercent != 0 {
		t.Fatalf("period.discountPercent = %.2f, want 0", period.discountPercent)
	}
}

func TestLoadUgenCurrencyRateFromEnv(t *testing.T) {
	t.Setenv("UGEN_CURRENCY_RATE_USD", "12650.25")

	got, err := loadUgenCurrencyRateFromEnv("USD")
	if err != nil {
		t.Fatal(err)
	}
	if got != 12650.25 {
		t.Fatalf("loadUgenCurrencyRateFromEnv() = %.2f, want 12650.25", got)
	}
}
