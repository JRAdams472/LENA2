package recipeimport

// Status is the single vocabulary for where a recipe import is in the
// pipeline. Every transition is enforced both here and in the conditional
// UPDATE statements in queries.sql, so a worker can never regress a
// terminal or admin-owned state.
type Status string

// Pipeline statuses. Worker-owned states run pending -> processing ->
// ocred -> drafted -> reviewing|ready; reviewing/ready are admin-owned;
// persisted and rejected are terminal; failed and profanity may retry.
const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusOCRED      Status = "ocred"
	StatusDrafted    Status = "drafted"
	StatusReviewing  Status = "reviewing"
	StatusReady      Status = "ready"
	StatusProfanity  Status = "profanity"
	StatusFailed     Status = "failed"
	StatusRejected   Status = "rejected"
	StatusPersisted  Status = "persisted"
)

// claimableStatuses are the states a worker may claim into processing.
// processing is deliberately excluded: a live claim must not be stolen, and
// orphaned processing rows are reset to pending at startup instead.
var claimableStatuses = []Status{StatusPending, StatusOCRED, StatusDrafted}

// pendingStatuses is the set shown in the admin review queue.
var pendingStatuses = []Status{
	StatusPending,
	StatusProcessing,
	StatusOCRED,
	StatusDrafted,
	StatusReviewing,
	StatusReady,
	StatusProfanity,
	StatusFailed,
}

// transitions is the allow-list of legal status changes. Anything not listed
// is rejected; the same predicates appear as WHERE clauses in queries.sql so
// the guard holds under concurrent writers.
var transitions = map[Status][]Status{
	StatusPending:    {StatusProcessing, StatusRejected},
	StatusProcessing: {StatusOCRED, StatusFailed, StatusProfanity, StatusRejected},
	StatusOCRED:      {StatusProcessing, StatusDrafted, StatusFailed, StatusProfanity, StatusRejected},
	StatusDrafted:    {StatusProcessing, StatusReviewing, StatusReady, StatusFailed, StatusProfanity, StatusRejected},
	StatusReviewing:  {StatusReady, StatusReviewing, StatusRejected, StatusPersisted},
	StatusReady:      {StatusReady, StatusReviewing, StatusRejected, StatusPersisted},
	StatusProfanity:  {StatusPending, StatusRejected},
	StatusFailed:     {StatusPending, StatusRejected},
	StatusRejected:   {},
	StatusPersisted:  {},
}

// canTransition reports whether moving from -> to is legal.
func canTransition(from, to Status) bool {
	for _, allowed := range transitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

// fromStatuses returns the statuses that may legally move to `to`, for use in
// SQL `status = ANY(...)` predicates.
func fromStatuses(to Status) []string {
	out := make([]string, 0, len(transitions))
	for from, tos := range transitions {
		for _, t := range tos {
			if t == to {
				out = append(out, string(from))
				break
			}
		}
	}
	return out
}

// statusStrings converts a Status slice for a sqlc varchar[] parameter.
func statusStrings(ss []Status) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = string(s)
	}
	return out
}
