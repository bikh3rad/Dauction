package handler_test

import "application/internal/entity"

// accessVIPApproved is a VIP+APPROVED access projection used by proxy tests to
// assert trusted header injection.
func accessVIPApproved() entity.Access {
	return entity.Access{
		ID:        "acc-9",
		Tier:      entity.TierVIP,
		KycStatus: entity.KycApproved,
		Eligible:  true,
	}
}
