package scimtest

import (
	"context"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/memsql/errors"
	"github.com/singlestore-labs/scim/scimerror"
)

// In-memory storage for SCIM users and groups
// we don't care SCIMID in in-memory storage for testing purpose
type Storage struct {
	mu     sync.RWMutex
	Users  map[SCIMUserID]SCIMUser
	Groups map[SCIMGroupID]SCIMGroup
}

func NewInMemStorage() *Storage {
	return &Storage{
		Users:  make(map[SCIMUserID]SCIMUser),
		Groups: make(map[SCIMGroupID]SCIMGroup),
	}
}

func (s *Storage) GetSCIMUser(ctx context.Context, _ string, userID string) (SCIMUser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.Users[SCIMUserID(userID)]
	if !ok {
		return SCIMUser{}, scimerror.NewSCIMErr(http.StatusNotFound, errors.Wrapf(scimerror.ErrNotFound, "user %s", userID))
	}
	return s.getGroupsForUser(user), nil
}

func (s *Storage) GetSCIMGroup(ctx context.Context, _ string, id string) (SCIMGroup, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	group, ok := s.Groups[SCIMGroupID(id)]
	if !ok {
		return SCIMGroup{}, scimerror.NewSCIMErr(http.StatusNotFound, errors.Wrapf(scimerror.ErrNotFound, "group %s", id))
	}
	return group, nil
}

func (s *Storage) GetAllSCIMUser(ctx context.Context, _ string) ([]SCIMUser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	results := make([]SCIMUser, 0, len(s.Users))
	for _, u := range s.Users {
		results = append(results, s.getGroupsForUser(u))
	}
	return results, nil
}

func (s *Storage) GetAllSCIMGroup(ctx context.Context, _ string) ([]SCIMGroup, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Collect(maps.Values(s.Groups)), nil
}

func (s *Storage) CreateSCIMUser(ctx context.Context, _ string, user SCIMUser) (SCIMUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.Users {
		if strings.EqualFold(existing.UserName, user.UserName) {
			return SCIMUser{}, scimerror.NewSCIMErr(http.StatusConflict, errors.Errorf("userName %q already exists", user.UserName))
		}
	}
	user.ID = uuid.New().String()
	user.Groups = nil
	s.Users[SCIMUserID(user.ID)] = user
	return user, nil
}

func (s *Storage) CreateSCIMGroup(ctx context.Context, _ string, group SCIMGroup) (SCIMGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.Groups {
		if strings.EqualFold(existing.DisplayName, group.DisplayName) && group.DisplayName != "" {
			return SCIMGroup{}, scimerror.NewSCIMErr(http.StatusConflict, errors.Errorf("displayName %q already exists", group.DisplayName))
		}
	}
	group.ID = uuid.New().String()
	s.Groups[SCIMGroupID(group.ID)] = group
	return group, nil
}

func (s *Storage) UpdateSCIMUser(ctx context.Context, _ string, id string, user SCIMUser) (SCIMUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.Users[SCIMUserID(id)]; !ok {
		return SCIMUser{}, scimerror.NewSCIMErr(http.StatusNotFound, errors.Wrapf(scimerror.ErrNotFound, "user %s", id))
	}
	user.Groups = nil
	user.ID = id
	s.Users[SCIMUserID(id)] = user
	return user, nil
}

func (s *Storage) UpdateSCIMGroup(ctx context.Context, _ string, id string, group SCIMGroup) (SCIMGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.Groups[SCIMGroupID(id)]; !ok {
		return SCIMGroup{}, scimerror.NewSCIMErr(http.StatusNotFound, errors.Wrapf(scimerror.ErrNotFound, "group %s", id))
	}
	group.ID = id
	s.Groups[SCIMGroupID(id)] = group
	return group, nil
}

func (s *Storage) DeleteSCIMUser(ctx context.Context, _ string, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.Users[SCIMUserID(id)]; !ok {
		return scimerror.NewSCIMErr(http.StatusNotFound, errors.Wrapf(scimerror.ErrNotFound, "user %s", id))
	}
	delete(s.Users, SCIMUserID(id))
	return nil
}

func (s *Storage) DeleteSCIMGroup(ctx context.Context, _ string, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.Groups[SCIMGroupID(id)]; !ok {
		return scimerror.NewSCIMErr(http.StatusNotFound, errors.Wrapf(scimerror.ErrNotFound, "group %s", id))
	}
	delete(s.Groups, SCIMGroupID(id))
	return nil
}

// SeedExampleData inserts one user and one group so Okta SPEC (which requires
// at least one existing user) and optional Groups checks can run.
func SeedExampleData(s *Storage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user := ExampleUserCore.Copy()
	user.ID = uuid.New().String()
	user.Groups = nil
	s.Users[SCIMUserID(user.ID)] = user

	groupID := uuid.New().String()
	s.Groups[SCIMGroupID(groupID)] = SCIMGroup{
		CoreGroup: CoreGroup{
			ID:          groupID,
			DisplayName: "Seed Group",
		},
	}
}

// getGroupsForUser get groups in SCIMUser when getting a user.
// Caller must hold s.mu.
func (s *Storage) getGroupsForUser(user SCIMUser) SCIMUser {
	groups := []Group{}
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
