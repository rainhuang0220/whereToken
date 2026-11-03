package cursor

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// History meta is the one-time association between an old Cursor
// observation and the account the user confirmed. The values are not
// secrets: a JWT, a path, and a raw subject hash are rejected or avoided.
const (
	HistoryDecisionMeta        = "cursor_history_decision"
	HistoryAuditSourceMeta     = "cursor_history_audit_source"
	HistoryAuditAtMeta         = "cursor_history_audit_at"
	HistoryBaselineMeta        = "cursor_history_baseline"
	HistoryDecisionSubjectMeta = "cursor_history_decision_subject"
	HistoryAccountHintMeta     = "cursor_history_account_hint"

	HistoryConfirmed    = "confirmed"
	HistoryDeclined     = "declined"
	HistoryAuditConfirm = "user_confirm"
	HistoryAuditDecline = "user_decline"

	HistoryEvidenceInsufficient = "insufficient"
	HistoryEvidenceRecorded     = "recorded"
	HistoryEvidenceConflict     = "conflict"

	KindHistoryConflict = "HISTORY_CONFLICT"
	KindHistoryHeld     = "HISTORY_HELD"
)

func HistoryConflictError() error { return usageKind(KindHistoryConflict) }

func HistoryHeldError() error { return usageKind(KindHistoryHeld) }

// AccountNamespace is the ledger source for observations taken after the
// user refused to attach an old history to this account. The raw subject
// is not the source id.
func AccountNamespace(subject string) string {
	sum := sha256.Sum256([]byte("cursor-history:" + subject))
	return "cursor#" + hex.EncodeToString(sum[:8])
}

// HistoryEvidence reports whether saved rows identify the current account.
// A verified snapshot, an API window, and a retained row are not account
// evidence. An empty result is insufficient, never a match.
func HistoryEvidence(hint, subject string, origins []string) string {
	hint = strings.TrimSpace(hint)
	recorded := hint != "" && hint == subject
	if hint != "" && hint != subject {
		return HistoryEvidenceConflict
	}
	for _, origin := range origins {
		switch origin {
		case "", "verified_snapshot", "api", "retained":
			continue
		default:
			if origin == subject {
				recorded = true
				continue
			}
			return HistoryEvidenceConflict
		}
	}
	if recorded {
		return HistoryEvidenceRecorded
	}
	return HistoryEvidenceInsufficient
}

// HistoryPrompt is the non-sensitive summary shown before a user answers.
type HistoryPrompt struct {
	Tokens   int64
	From, To string
	Days     int
	Evidence string
	Subject  string
}

// FormatHistoryPrompt renders the one-time association question.
// The text has no session material and does not name a conflicting account.
func FormatHistoryPrompt(v HistoryPrompt) string {
	var b strings.Builder
	b.WriteString("这份 Cursor 历史还没有账号主体。\n")
	switch v.Evidence {
	case HistoryEvidenceRecorded:
		b.WriteString("已保存的记录与当前账号一致。仍需要你明确确认才会关联。\n")
	case HistoryEvidenceConflict:
		b.WriteString("已保存的记录指向另一个账号。这次不能关联。\n")
	default:
		b.WriteString("已检查快照和账本记录，没有足够证据把这份历史关联到当前账号。\n")
	}
	fmt.Fprintf(&b, "历史 token：%d\n", v.Tokens)
	fmt.Fprintf(&b, "日期：%s..%s\n", v.From, v.To)
	fmt.Fprintf(&b, "已记录日数：%d\n", v.Days)
	fmt.Fprintf(&b, "当前账号：%s\n", PublicSubject(v.Subject))
	switch v.Evidence {
	case HistoryEvidenceConflict:
		b.WriteString("输入 no 保持旧历史独立，当前账号以后单独记录。输入 yes 不会关联。\n")
	default:
		b.WriteString("输入 yes 关联。输入 no 保持旧历史独立，当前账号以后单独记录。\n")
	}
	return b.String()
}

// PublicSubject is the identity printed to the user. A JWT-shaped value
// is replaced so the prompt does not become a credential.
func PublicSubject(subject string) string {
	subject = strings.TrimSpace(subject)
	if subject == "" || len(subject) > 80 || strings.Contains(strings.ToLower(subject), "eyj") || strings.Count(subject, ".") >= 2 {
		return "当前 Cursor 账号"
	}
	return subject
}

// ParseHistoryAnswer accepts only the whole words yes and no.
func ParseHistoryAnswer(line string) (string, bool) {
	switch strings.TrimSpace(line) {
	case "yes":
		return HistoryConfirmed, true
	case "no":
		return HistoryDeclined, true
	default:
		return "", false
	}
}
