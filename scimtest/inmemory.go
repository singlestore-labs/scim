package scimtest

import (
	"context"
	"maps"
	"slices"

	"github.com/google/uuid"
)

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

func (s Storage) GetSCIMUser(ctx context.Context, id string) (SCIMUser, error) {
	return s.getGroupsForUser(s.Users[SCIMUserID(id)]), nil
}

func (s Storage) GetSCIMGroup(ctx context.Context, id string) (SCIMGroup, error) {
	return s.Groups[SCIMGroupID(id)], nil
}

func (s Storage) GetAllSCIMUser(ctx context.Context) ([]SCIMUser, error) {
	results := make([]SCIMUser, 0, len(s.Users))
	for _, u := range s.Users {
		results = append(results, s.getGroupsForUser(u))
	}
	return results, nil
}

func (s Storage) GetAllSCIMGroup(ctx context.Context) ([]SCIMGroup, error) {
	return slices.Collect(maps.Values(s.Groups)), nil
}

func (s *Storage) CreateSCIMUser(ctx context.Context, user SCIMUser) (SCIMUser, error) {
	user.ID = uuid.New().String()
	user.Groups = nil
	s.Users[SCIMUserID(user.ID)] = user
	return user, nil
}

func (s *Storage) CreateSCIMGroup(ctx context.Context, group SCIMGroup) (SCIMGroup, error) {
	group.ID = uuid.New().String()
	s.Groups[SCIMGroupID(group.ID)] = group
	return group, nil
}

func (s *Storage) UpdateSCIMUser(ctx context.Context, id string, user SCIMUser) (SCIMUser, error) {
	user.Groups = nil
	user.ID = id
	s.Users[SCIMUserID(id)] = user
	return user, nil
}

func (s *Storage) UpdateSCIMGroup(ctx context.Context, id string, group SCIMGroup) (SCIMGroup, error) {
	s.Groups[SCIMGroupID(group.ID)] = group
	return group, nil
}

func (s *Storage) DeleteSCIMUser(ctx context.Context, id string) error {
	delete(s.Users, SCIMUserID(id))
	return nil
}

func (s *Storage) DeleteSCIMGroup(ctx context.Context, id string) error {
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
