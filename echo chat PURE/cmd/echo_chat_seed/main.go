package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"hash/crc32"
	"os"
	"strings"
	"time"

	"gorm.io/gorm"

	"echo_chat_server/internal/dao"
	"echo_chat_server/internal/model"
	myredis "echo_chat_server/internal/service/redis"
	"echo_chat_server/pkg/enum/contact/contact_status_enum"
	"echo_chat_server/pkg/enum/contact/contact_type_enum"
	"echo_chat_server/pkg/enum/contact_apply/contact_apply_status_enum"
	"echo_chat_server/pkg/enum/group_info/add_mode_enum"
	"echo_chat_server/pkg/enum/group_info/group_status_enum"
	"echo_chat_server/pkg/enum/message/message_status_enum"
	"echo_chat_server/pkg/enum/message/message_type_enum"
	"echo_chat_server/pkg/enum/user_info/user_status_enum"
)

const (
	defaultAvatar = "https://cube.elemecdn.com/0/88/03b0d39583f48206768a7534e55bcpng.png"
	timeFormat    = "2006-01-02 15:04:05"
)

type seedOptions struct {
	Prefix            string
	UserCount         int
	AdminCount        int
	GroupCount        int
	GroupSize         int
	FriendSpan        int
	PairMessages      int
	GroupMessages     int
	ApplyCount        int
	Password          string
	TelephoneStart    int64
	ResetPrefixBefore bool
}

type seedSummary struct {
	Prefix            string `json:"prefix"`
	UserCount         int    `json:"user_count"`
	AdminCount        int    `json:"admin_count"`
	GroupCount        int    `json:"group_count"`
	GroupSize         int    `json:"group_size"`
	FriendPairCount   int    `json:"friend_pair_count"`
	UserContactCount  int    `json:"user_contact_count"`
	SessionCount      int    `json:"session_count"`
	MessageCount      int    `json:"message_count"`
	ApplyCount        int    `json:"apply_count"`
	ResetPrefixBefore bool   `json:"reset_prefix_before"`
	GeneratedAt       string `json:"generated_at"`
}

type seedUser struct {
	Model model.UserInfo
}

type seedGroup struct {
	Model   model.GroupInfo
	Members []seedUser
}

type userPair struct {
	Left  seedUser
	Right seedUser
}

type seeder struct {
	opts       seedOptions
	idWidth    int
	users      []seedUser
	groups     []seedGroup
	friendPair []userPair
}

type progressEvent struct {
	Type    string `json:"type"`
	Scope   string `json:"scope,omitempty"`
	Current int    `json:"current"`
	Total   int    `json:"total"`
	Step    string `json:"step"`
	Detail  string `json:"detail,omitempty"`
}

func main() {
	opts, err := parseFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "seed options invalid: %v\n", err)
		os.Exit(1)
	}

	s := &seeder{
		opts:    opts,
		idWidth: 19 - len(opts.Prefix),
	}

	summary, err := s.run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "seed failed: %v\n", err)
		os.Exit(1)
	}

	output, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal summary failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(output))
}

func parseFlags() (seedOptions, error) {
	var opts seedOptions
	flag.StringVar(&opts.Prefix, "prefix", "K6", "business id prefix for generated data")
	flag.IntVar(&opts.UserCount, "user-count", 200, "number of users to generate")
	flag.IntVar(&opts.AdminCount, "admin-count", 1, "number of admin users to generate")
	flag.IntVar(&opts.GroupCount, "group-count", 12, "number of groups to generate")
	flag.IntVar(&opts.GroupSize, "group-size", 25, "number of members in each group")
	flag.IntVar(&opts.FriendSpan, "friend-span", 10, "each user will be connected with the next N users")
	flag.IntVar(&opts.PairMessages, "pair-messages", 30, "messages per direct friendship pair")
	flag.IntVar(&opts.GroupMessages, "group-messages", 120, "messages per group")
	flag.IntVar(&opts.ApplyCount, "apply-count", 40, "pending contact apply records to generate")
	flag.StringVar(&opts.Password, "password", "123456", "password for generated users")
	flag.Int64Var(&opts.TelephoneStart, "telephone-start", 0, "starting telephone number, default derives a unique range from prefix")
	flag.BoolVar(&opts.ResetPrefixBefore, "reset-prefix", true, "delete existing generated data with the same prefix before seeding")
	flag.Parse()

	opts.Prefix = sanitizePrefix(opts.Prefix)
	switch {
	case opts.Prefix == "":
		return opts, fmt.Errorf("prefix cannot be empty after sanitization")
	case len(opts.Prefix) > 10:
		return opts, fmt.Errorf("prefix too long")
	case 19-len(opts.Prefix) < 6:
		return opts, fmt.Errorf("prefix too long for 20-char business ids")
	case opts.UserCount < 2:
		return opts, fmt.Errorf("user-count must be at least 2")
	case opts.AdminCount < 0:
		return opts, fmt.Errorf("admin-count cannot be negative")
	case opts.AdminCount > opts.UserCount:
		return opts, fmt.Errorf("admin-count cannot exceed user-count")
	case opts.GroupCount < 0:
		return opts, fmt.Errorf("group-count cannot be negative")
	case opts.GroupSize < 1:
		return opts, fmt.Errorf("group-size must be at least 1")
	case opts.FriendSpan < 1:
		return opts, fmt.Errorf("friend-span must be at least 1")
	case opts.PairMessages < 0:
		return opts, fmt.Errorf("pair-messages cannot be negative")
	case opts.GroupMessages < 0:
		return opts, fmt.Errorf("group-messages cannot be negative")
	case opts.ApplyCount < 0:
		return opts, fmt.Errorf("apply-count cannot be negative")
	}

	if opts.TelephoneStart <= 0 {
		opts.TelephoneStart = defaultTelephoneStart(opts.Prefix)
	}

	if opts.GroupSize > opts.UserCount {
		opts.GroupSize = opts.UserCount
	}
	if opts.FriendSpan >= opts.UserCount {
		opts.FriendSpan = opts.UserCount - 1
	}
	return opts, nil
}

func sanitizePrefix(raw string) string {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	var builder strings.Builder
	for _, r := range raw {
		if r >= 'A' && r <= 'Z' {
			builder.WriteRune(r)
		}
		if r >= '0' && r <= '9' {
			builder.WriteRune(r)
		}
	}
	result := builder.String()
	if len(result) > 6 {
		result = result[:6]
	}
	return result
}

func defaultTelephoneStart(prefix string) int64 {
	// Reserve 1000 numbers per prefix and spread prefixes across a deterministic mobile range.
	slot := int64(crc32.ChecksumIEEE([]byte(prefix)) % 1_000_000)
	return 17000000000 + slot*1000
}

func (s *seeder) run() (*seedSummary, error) {
	s.emitProgress(1, 9, "prepare_users", fmt.Sprintf("user_count=%d", s.opts.UserCount))
	s.prepareUsers()
	s.emitProgress(2, 9, "prepare_groups", fmt.Sprintf("group_count=%d group_size=%d", s.opts.GroupCount, s.opts.GroupSize))
	s.prepareGroups()
	s.emitProgress(3, 9, "prepare_friend_pairs", fmt.Sprintf("friend_span=%d", s.opts.FriendSpan))
	s.prepareFriendPairs()

	var summary *seedSummary
	err := dao.GormDB.Transaction(func(tx *gorm.DB) error {
		if s.opts.ResetPrefixBefore {
			s.emitProgress(4, 9, "cleanup_existing", fmt.Sprintf("prefix=%s", s.opts.Prefix))
			if err := s.cleanupExisting(tx); err != nil {
				return err
			}
		}
		s.emitProgress(5, 9, "insert_users", fmt.Sprintf("count=%d", len(s.users)))
		if err := s.insertUsers(tx); err != nil {
			return err
		}
		s.emitProgress(6, 9, "insert_groups", fmt.Sprintf("count=%d", len(s.groups)))
		if err := s.insertGroups(tx); err != nil {
			return err
		}

		friendSet := s.friendSet()
		s.emitProgress(7, 9, "insert_contacts_and_sessions", fmt.Sprintf("friend_pairs=%d", len(s.friendPair)))
		contactCount, sessionCount, err := s.insertContactsAndSessions(tx)
		if err != nil {
			return err
		}
		s.emitProgress(8, 9, "insert_contact_applies", fmt.Sprintf("apply_count=%d", s.opts.ApplyCount))
		applyCount, err := s.insertContactApplies(tx, friendSet)
		if err != nil {
			return err
		}
		s.emitProgress(9, 9, "insert_messages", fmt.Sprintf("pair_messages=%d group_messages=%d", s.opts.PairMessages, s.opts.GroupMessages))
		messageCount, err := s.insertMessages(tx)
		if err != nil {
			return err
		}

		summary = &seedSummary{
			Prefix:            s.opts.Prefix,
			UserCount:         len(s.users),
			AdminCount:        s.opts.AdminCount,
			GroupCount:        len(s.groups),
			GroupSize:         s.opts.GroupSize,
			FriendPairCount:   len(s.friendPair),
			UserContactCount:  contactCount,
			SessionCount:      sessionCount,
			MessageCount:      messageCount,
			ApplyCount:        applyCount,
			ResetPrefixBefore: s.opts.ResetPrefixBefore,
			GeneratedAt:       time.Now().Format(timeFormat),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := clearRuntimeCaches(); err != nil {
		return nil, err
	}
	s.emitProgress(9, 9, "done", "seed completed")
	return summary, nil
}

func (s *seeder) emitProgress(current int, total int, step string, detail string) {
	event := progressEvent{
		Type:    "seed_progress",
		Scope:   "stage",
		Current: current,
		Total:   total,
		Step:    step,
		Detail:  detail,
	}
	payload, err := json.Marshal(event)
	if err != nil {
		fmt.Printf("[seed] progress %d/%d %s %s\n", current, total, step, detail)
		return
	}
	fmt.Println(string(payload))
}

func (s *seeder) emitBatchProgress(step string, current int, total int, detail string) {
	event := progressEvent{
		Type:    "seed_progress",
		Scope:   "batch",
		Current: current,
		Total:   total,
		Step:    step,
		Detail:  detail,
	}
	payload, err := json.Marshal(event)
	if err != nil {
		fmt.Printf("[seed] batch %d/%d %s %s\n", current, total, step, detail)
		return
	}
	fmt.Println(string(payload))
}

func totalBatches(total int, batchSize int) int {
	if total <= 0 || batchSize <= 0 {
		return 0
	}
	return (total + batchSize - 1) / batchSize
}

func (s *seeder) prepareUsers() {
	s.users = make([]seedUser, 0, s.opts.UserCount)
	baseTime := time.Now().Add(-30 * 24 * time.Hour)
	for i := 1; i <= s.opts.UserCount; i++ {
		telephone := fmt.Sprintf("%011d", s.opts.TelephoneStart+int64(i))
		user := model.UserInfo{
			Uuid:      s.businessID("U", i),
			Nickname:  fmt.Sprintf("%s_u%04d", strings.ToLower(s.opts.Prefix), i),
			Telephone: telephone,
			Email:     fmt.Sprintf("%s_u%04d@example.com", strings.ToLower(s.opts.Prefix), i),
			Avatar:    defaultAvatar,
			Gender:    int8(i % 2),
			Signature: fmt.Sprintf("seed user %04d for pressure testing", i),
			Password:  s.opts.Password,
			Birthday:  fmt.Sprintf("1990%02d%02d", (i%12)+1, (i%28)+1),
			CreatedAt: baseTime.Add(time.Duration(i) * time.Minute),
			IsAdmin:   0,
			Status:    user_status_enum.NORMAL,
			LastOnlineAt: sql.NullTime{
				Time:  baseTime.Add(time.Duration(i) * time.Minute),
				Valid: true,
			},
		}
		if i <= s.opts.AdminCount {
			user.IsAdmin = 1
		}
		s.users = append(s.users, seedUser{Model: user})
	}
}

func (s *seeder) prepareGroups() {
	s.groups = make([]seedGroup, 0, s.opts.GroupCount)
	baseTime := time.Now().Add(-20 * 24 * time.Hour)
	for i := 1; i <= s.opts.GroupCount; i++ {
		owner := s.users[(i-1)%len(s.users)]
		members := make([]seedUser, 0, s.opts.GroupSize)
		memberSeen := make(map[string]struct{}, s.opts.GroupSize)
		addMember := func(user seedUser) {
			if _, ok := memberSeen[user.Model.Uuid]; ok {
				return
			}
			memberSeen[user.Model.Uuid] = struct{}{}
			members = append(members, user)
		}
		addMember(owner)
		for offset := 1; len(members) < s.opts.GroupSize; offset++ {
			next := s.users[(i-1+offset)%len(s.users)]
			addMember(next)
		}

		memberIDs := make([]string, 0, len(members))
		for _, member := range members {
			memberIDs = append(memberIDs, member.Model.Uuid)
		}
		memberPayload, _ := json.Marshal(memberIDs)
		group := model.GroupInfo{
			Uuid:      s.businessID("G", i),
			Name:      fmt.Sprintf("%s_group_%03d", strings.ToLower(s.opts.Prefix), i),
			Notice:    fmt.Sprintf("seed group %03d for pressure testing", i),
			Members:   memberPayload,
			MemberCnt: len(memberIDs),
			OwnerId:   owner.Model.Uuid,
			AddMode:   add_mode_enum.DIRECT,
			Avatar:    defaultAvatar,
			Status:    group_status_enum.NORMAL,
			CreatedAt: baseTime.Add(time.Duration(i) * time.Hour),
			UpdatedAt: baseTime.Add(time.Duration(i) * time.Hour),
		}
		s.groups = append(s.groups, seedGroup{Model: group, Members: members})
	}
}

func (s *seeder) prepareFriendPairs() {
	s.friendPair = make([]userPair, 0)
	for i := 0; i < len(s.users); i++ {
		for step := 1; step <= s.opts.FriendSpan && i+step < len(s.users); step++ {
			s.friendPair = append(s.friendPair, userPair{
				Left:  s.users[i],
				Right: s.users[i+step],
			})
		}
	}
}

func (s *seeder) insertUsers(tx *gorm.DB) error {
	users := make([]model.UserInfo, 0, len(s.users))
	for _, user := range s.users {
		users = append(users, user.Model)
	}
	const batchSize = 200
	total := totalBatches(len(users), batchSize)
	for batchIndex, start := 0, 0; start < len(users); batchIndex, start = batchIndex+1, start+batchSize {
		end := start + batchSize
		if end > len(users) {
			end = len(users)
		}
		s.emitBatchProgress("insert_users_batch", batchIndex+1, total, fmt.Sprintf("rows=%d", end-start))
		if err := tx.CreateInBatches(users[start:end], batchSize).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *seeder) insertGroups(tx *gorm.DB) error {
	groups := make([]model.GroupInfo, 0, len(s.groups))
	for _, group := range s.groups {
		groups = append(groups, group.Model)
	}
	if len(groups) == 0 {
		return nil
	}
	const batchSize = 100
	total := totalBatches(len(groups), batchSize)
	for batchIndex, start := 0, 0; start < len(groups); batchIndex, start = batchIndex+1, start+batchSize {
		end := start + batchSize
		if end > len(groups) {
			end = len(groups)
		}
		s.emitBatchProgress("insert_groups_batch", batchIndex+1, total, fmt.Sprintf("rows=%d", end-start))
		if err := tx.CreateInBatches(groups[start:end], batchSize).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *seeder) insertContactsAndSessions(tx *gorm.DB) (int, int, error) {
	contacts := make([]model.UserContact, 0, len(s.friendPair)*2+len(s.groups)*s.opts.GroupSize)
	sessions := make([]model.Session, 0, len(s.friendPair)*2+len(s.groups)*s.opts.GroupSize)

	now := time.Now().Add(-10 * 24 * time.Hour)
	sessionSeq := 1
	for _, pair := range s.friendPair {
		contacts = append(contacts,
			model.UserContact{
				UserId:      pair.Left.Model.Uuid,
				ContactId:   pair.Right.Model.Uuid,
				ContactType: contact_type_enum.USER,
				Status:      contact_status_enum.NORMAL,
				CreatedAt:   now,
				UpdateAt:    now,
			},
			model.UserContact{
				UserId:      pair.Right.Model.Uuid,
				ContactId:   pair.Left.Model.Uuid,
				ContactType: contact_type_enum.USER,
				Status:      contact_status_enum.NORMAL,
				CreatedAt:   now,
				UpdateAt:    now,
			},
		)

		sessions = append(sessions,
			model.Session{
				Uuid:        s.businessID("S", sessionSeq),
				SendId:      pair.Left.Model.Uuid,
				ReceiveId:   pair.Right.Model.Uuid,
				ReceiveName: pair.Right.Model.Nickname,
				Avatar:      pair.Right.Model.Avatar,
				LastMessage: "seed direct session ready",
				LastMessageAt: sql.NullTime{
					Time:  now,
					Valid: true,
				},
				CreatedAt: now,
			},
		)
		sessionSeq++
		sessions = append(sessions,
			model.Session{
				Uuid:        s.businessID("S", sessionSeq),
				SendId:      pair.Right.Model.Uuid,
				ReceiveId:   pair.Left.Model.Uuid,
				ReceiveName: pair.Left.Model.Nickname,
				Avatar:      pair.Left.Model.Avatar,
				LastMessage: "seed direct session ready",
				LastMessageAt: sql.NullTime{
					Time:  now,
					Valid: true,
				},
				CreatedAt: now,
			},
		)
		sessionSeq++
		now = now.Add(time.Second)
	}

	for _, group := range s.groups {
		for _, member := range group.Members {
			contacts = append(contacts, model.UserContact{
				UserId:      member.Model.Uuid,
				ContactId:   group.Model.Uuid,
				ContactType: contact_type_enum.GROUP,
				Status:      contact_status_enum.NORMAL,
				CreatedAt:   now,
				UpdateAt:    now,
			})
			sessions = append(sessions, model.Session{
				Uuid:        s.businessID("S", sessionSeq),
				SendId:      member.Model.Uuid,
				ReceiveId:   group.Model.Uuid,
				ReceiveName: group.Model.Name,
				Avatar:      group.Model.Avatar,
				LastMessage: "seed group session ready",
				LastMessageAt: sql.NullTime{
					Time:  now,
					Valid: true,
				},
				CreatedAt: now,
			})
			sessionSeq++
			now = now.Add(time.Second)
		}
	}

	if len(contacts) > 0 {
		const batchSize = 1000
		total := totalBatches(len(contacts), batchSize)
		for batchIndex, start := 0, 0; start < len(contacts); batchIndex, start = batchIndex+1, start+batchSize {
			end := start + batchSize
			if end > len(contacts) {
				end = len(contacts)
			}
			s.emitBatchProgress("insert_contacts_batch", batchIndex+1, total, fmt.Sprintf("rows=%d", end-start))
			if err := tx.CreateInBatches(contacts[start:end], batchSize).Error; err != nil {
				return 0, 0, err
			}
		}
	}
	if len(sessions) > 0 {
		const batchSize = 1000
		total := totalBatches(len(sessions), batchSize)
		for batchIndex, start := 0, 0; start < len(sessions); batchIndex, start = batchIndex+1, start+batchSize {
			end := start + batchSize
			if end > len(sessions) {
				end = len(sessions)
			}
			s.emitBatchProgress("insert_sessions_batch", batchIndex+1, total, fmt.Sprintf("rows=%d", end-start))
			if err := tx.CreateInBatches(sessions[start:end], batchSize).Error; err != nil {
				return 0, 0, err
			}
		}
	}
	return len(contacts), len(sessions), nil
}

func (s *seeder) insertContactApplies(tx *gorm.DB, friendSet map[string]struct{}) (int, error) {
	if s.opts.ApplyCount == 0 {
		return 0, nil
	}
	applies := make([]model.ContactApply, 0, s.opts.ApplyCount)
	now := time.Now().Add(-48 * time.Hour)
	applySeq := 1
	for offset := s.opts.FriendSpan + 1; offset < len(s.users) && len(applies) < s.opts.ApplyCount; offset++ {
		for i := 0; i+offset < len(s.users) && len(applies) < s.opts.ApplyCount; i++ {
			left := s.users[i]
			right := s.users[i+offset]
			if _, ok := friendSet[s.normalizePair(left.Model.Uuid, right.Model.Uuid)]; ok {
				continue
			}
			applies = append(applies, model.ContactApply{
				Uuid:        s.businessID("A", applySeq),
				UserId:      left.Model.Uuid,
				ContactId:   right.Model.Uuid,
				ContactType: contact_type_enum.USER,
				Status:      contact_apply_status_enum.PENDING,
				Message:     fmt.Sprintf("seed apply from %s to %s", left.Model.Nickname, right.Model.Nickname),
				LastApplyAt: now,
			})
			applySeq++
			now = now.Add(time.Minute)
		}
	}
	if len(applies) == 0 {
		return 0, nil
	}
	const batchSize = 500
	total := totalBatches(len(applies), batchSize)
	for batchIndex, start := 0, 0; start < len(applies); batchIndex, start = batchIndex+1, start+batchSize {
		end := start + batchSize
		if end > len(applies) {
			end = len(applies)
		}
		s.emitBatchProgress("insert_contact_applies_batch", batchIndex+1, total, fmt.Sprintf("rows=%d", end-start))
		if err := tx.CreateInBatches(applies[start:end], batchSize).Error; err != nil {
			return 0, err
		}
	}
	return len(applies), nil
}

func (s *seeder) insertMessages(tx *gorm.DB) (int, error) {
	type sessionRef struct {
		UUID string
	}

	var sessionRows []struct {
		Uuid      string
		SendId    string
		ReceiveId string
	}
	if err := tx.Model(&model.Session{}).
		Select("uuid", "send_id", "receive_id").
		Where("send_id LIKE ? OR receive_id LIKE ? OR receive_id LIKE ?", s.likePrefix("U"), s.likePrefix("U"), s.likePrefix("G")).
		Find(&sessionRows).Error; err != nil {
		return 0, err
	}

	directSessionMap := make(map[string]sessionRef, len(sessionRows))
	groupSessionMap := make(map[string]sessionRef, len(sessionRows))
	for _, session := range sessionRows {
		key := session.SendId + "|" + session.ReceiveId
		if strings.HasPrefix(session.ReceiveId, "G") {
			groupSessionMap[key] = sessionRef{UUID: session.Uuid}
			continue
		}
		directSessionMap[key] = sessionRef{UUID: session.Uuid}
	}

	messages := make([]model.Message, 0, len(s.friendPair)*s.opts.PairMessages+len(s.groups)*s.opts.GroupMessages)
	now := time.Now().Add(-7 * 24 * time.Hour)
	messageSeq := 1

	for _, pair := range s.friendPair {
		for i := 1; i <= s.opts.PairMessages; i++ {
			sender := pair.Left
			receiver := pair.Right
			if i%2 == 0 {
				sender, receiver = receiver, sender
			}
			sessionID := directSessionMap[sender.Model.Uuid+"|"+receiver.Model.Uuid].UUID
				messages = append(messages, model.Message{
					Uuid:       s.businessID("M", messageSeq),
					SessionId:  sessionID,
					Type:       message_type_enum.Text,
					Content:    fmt.Sprintf("seed direct message %03d from %s to %s", i, sender.Model.Nickname, receiver.Model.Nickname),
					SendId:     sender.Model.Uuid,
					SendName:   sender.Model.Nickname,
					SendAvatar: sender.Model.Avatar,
					ReceiveId:  receiver.Model.Uuid,
					ConversationKey: model.BuildConversationKey(
						sender.Model.Uuid,
						receiver.Model.Uuid,
					),
					FileType:   "",
					FileName:   "",
					FileSize:   "0B",
					Status:     message_status_enum.Sent,
				SessionSeq: int64(i),
				CreatedAt:  now,
				SendAt: sql.NullTime{
					Time:  now,
					Valid: true,
				},
			})
			messageSeq++
			now = now.Add(time.Second)
		}
	}

	for _, group := range s.groups {
		for i := 1; i <= s.opts.GroupMessages; i++ {
			sender := group.Members[(i-1)%len(group.Members)]
			sessionID := groupSessionMap[sender.Model.Uuid+"|"+group.Model.Uuid].UUID
				messages = append(messages, model.Message{
					Uuid:       s.businessID("M", messageSeq),
					SessionId:  sessionID,
					Type:       message_type_enum.Text,
					Content:    fmt.Sprintf("seed group message %03d in %s from %s", i, group.Model.Name, sender.Model.Nickname),
					SendId:     sender.Model.Uuid,
					SendName:   sender.Model.Nickname,
					SendAvatar: sender.Model.Avatar,
					ReceiveId:  group.Model.Uuid,
					ConversationKey: model.BuildConversationKey(
						sender.Model.Uuid,
						group.Model.Uuid,
					),
					FileType:   "",
					FileName:   "",
					FileSize:   "0B",
					Status:     message_status_enum.Sent,
				SessionSeq: int64(i),
				CreatedAt:  now,
				SendAt: sql.NullTime{
					Time:  now,
					Valid: true,
				},
			})
			messageSeq++
			now = now.Add(time.Second)
		}
	}

	if len(messages) == 0 {
		return 0, nil
	}
	const batchSize = 1000
	total := totalBatches(len(messages), batchSize)
	for batchIndex, start := 0, 0; start < len(messages); batchIndex, start = batchIndex+1, start+batchSize {
		end := start + batchSize
		if end > len(messages) {
			end = len(messages)
		}
		s.emitBatchProgress("insert_messages_batch", batchIndex+1, total, fmt.Sprintf("rows=%d", end-start))
		if err := tx.CreateInBatches(messages[start:end], batchSize).Error; err != nil {
			return 0, err
		}
	}
	return len(messages), nil
}

func (s *seeder) cleanupExisting(tx *gorm.DB) error {
	userLike := s.likePrefix("U")
	groupLike := s.likePrefix("G")
	sessionLike := s.likePrefix("S")
	messageLike := s.likePrefix("M")
	applyLike := s.likePrefix("A")

	steps := []struct {
		model any
		query func(*gorm.DB) *gorm.DB
	}{
		{
			model: &model.Message{},
			query: func(db *gorm.DB) *gorm.DB {
				return db.Where(
					"uuid LIKE ? OR session_id LIKE ? OR send_id LIKE ? OR receive_id LIKE ? OR receive_id LIKE ?",
					messageLike, sessionLike, userLike, userLike, groupLike,
				)
			},
		},
		{
			model: &model.ContactApply{},
			query: func(db *gorm.DB) *gorm.DB {
				return db.Where("uuid LIKE ? OR user_id LIKE ? OR contact_id LIKE ? OR contact_id LIKE ?", applyLike, userLike, userLike, groupLike)
			},
		},
		{
			model: &model.Session{},
			query: func(db *gorm.DB) *gorm.DB {
				return db.Where("uuid LIKE ? OR send_id LIKE ? OR receive_id LIKE ? OR receive_id LIKE ?", sessionLike, userLike, userLike, groupLike)
			},
		},
		{
			model: &model.UserContact{},
			query: func(db *gorm.DB) *gorm.DB {
				return db.Where("user_id LIKE ? OR contact_id LIKE ? OR contact_id LIKE ?", userLike, userLike, groupLike)
			},
		},
		{
			model: &model.GroupInfo{},
			query: func(db *gorm.DB) *gorm.DB {
				return db.Where("uuid LIKE ? OR owner_id LIKE ?", groupLike, userLike)
			},
		},
		{
			model: &model.UserInfo{},
			query: func(db *gorm.DB) *gorm.DB {
				return db.Where("uuid LIKE ? OR nickname LIKE ?", userLike, strings.ToLower(s.opts.Prefix)+"_%")
			},
		},
	}

	for _, step := range steps {
		if err := step.query(tx.Unscoped().Model(step.model)).Delete(step.model).Error; err != nil {
			return err
		}
	}
	return nil
}

func clearRuntimeCaches() error {
	patterns := []string{
		"contact_user_list_*",
		"my_joined_group_list_*",
		"session_list_*",
		"group_session_list_*",
		"message_list_*",
		"group_messagelist_*",
		"session_*",
		"user_info_*",
		"message_session_seq_*",
	}
	for _, pattern := range patterns {
		if err := myredis.DelKeysWithPattern(pattern); err != nil {
			return err
		}
	}
	return nil
}

func (s *seeder) friendSet() map[string]struct{} {
	result := make(map[string]struct{}, len(s.friendPair))
	for _, pair := range s.friendPair {
		result[s.normalizePair(pair.Left.Model.Uuid, pair.Right.Model.Uuid)] = struct{}{}
	}
	return result
}

func (s *seeder) normalizePair(left, right string) string {
	if left < right {
		return left + "|" + right
	}
	return right + "|" + left
}

func (s *seeder) businessID(kind string, seq int) string {
	return fmt.Sprintf("%s%s%0*d", kind, s.opts.Prefix, s.idWidth, seq)
}

func (s *seeder) likePrefix(kind string) string {
	return kind + s.opts.Prefix + "%"
}
