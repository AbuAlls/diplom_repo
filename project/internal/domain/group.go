package domain

import "time"

// Group is a corporate account: a shared workspace. Every member of a group can
// see every other member's plans and documents ("all members share
// everything"). CreatedBy is the member who created the group and may add others.
type Group struct {
	ID          int64
	Name        string
	Description string
	Role        string
	CreatedBy   *int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// GroupMember is a user's membership in a group, joined with the user's
// identity for listing.
type GroupMember struct {
	UserID   int64
	GroupID  int64
	Email    string
	FullName string
	JoinedAt time.Time
}
