package scimtest

import (
	"context"
	"maps"
	"slices"

	"github.com/google/uuid"
)

// In-memory storage for SCIM users and groups
// we don't care SCIMID in in-memory storage for testing purpose
type Storage struct {
	Users  map[SCIMUserID]SCIMUser
	Groups map[SCIMGroupID]SCIMGroup
}

func NewInMemStorage() *Storage {
	return &Storage{
		Users:  make(map[SCIMUserID]SCIMUser),
		Groups: make(map[SCIMGroupID]SCIMGroup),
	}
}

func (s Storage) GetSCIMUser(ctx context.Context, _ string, userID string) (SCIMUser, error) {
	return s.getGroupsForUser(s.Users[SCIMUserID(userID)]), nil
}

func (s Storage) GetSCIMGroup(ctx context.Context, _ string, id string) (SCIMGroup, error) {
	return s.Groups[SCIMGroupID(id)], nil
}

func (s Storage) GetAllSCIMUser(ctx context.Context, _ string) ([]SCIMUser, error) {
	results := make([]SCIMUser, 0, len(s.Users))
	for _, u := range s.Users {
		results = append(results, s.getGroupsForUser(u))
	}
	return results, nil
}

func (s Storage) GetAllSCIMGroup(ctx context.Context, _ string) ([]SCIMGroup, error) {
	return slices.Collect(maps.Values(s.Groups)), nil
}

func (s *Storage) CreateSCIMUser(ctx context.Context, _ string, user SCIMUser) (SCIMUser, error) {
	user.ID = uuid.New().String()
	user.Groups = nil
	s.Users[SCIMUserID(user.ID)] = user
	return user, nil
}

func (s *Storage) CreateSCIMGroup(ctx context.Context, _ string, group SCIMGroup) (SCIMGroup, error) {
	group.ID = uuid.New().String()
	s.Groups[SCIMGroupID(group.ID)] = group
	return group, nil
}

func (s *Storage) UpdateSCIMUser(ctx context.Context, _ string, id string, user SCIMUser) (SCIMUser, error) {
	user.Groups = nil
	user.ID = id
	s.Users[SCIMUserID(id)] = user
	return user, nil
}

func (s *Storage) UpdateSCIMGroup(ctx context.Context, _ string, id string, group SCIMGroup) (SCIMGroup, error) {
	s.Groups[SCIMGroupID(group.ID)] = group
	return group, nil
}

func (s *Storage) DeleteSCIMUser(ctx context.Context, _ string, id string) error {
	delete(s.Users, SCIMUserID(id))
	return nil
}

func (s *Storage) DeleteSCIMGroup(ctx context.Context, _ string, id string) error {
	delete(s.Groups, SCIMGroupID(id))
	return nil
}

// getGroupsForUser get groups in SCIMUser when getting a user
func (s *Storage) getGroupsForUser(user SCIMUser) SCIMUser {
	groups := []Group{}
	// remove user from all groups first
	for gid, group := range s.Groups {
		for _, member := range group.Members {
			if member.Value == user.ID {
				groups = append(groups, Group{Value: string(gid), Display: group.DisplayName, Ref: "https://example.com/v2/Groups/" + string(gid)})
			}
		}
	}
	user.Groups = groups
	return user
}
