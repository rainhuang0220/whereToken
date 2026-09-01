package metric

import (
	"sort"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/price"
)

const (
	unlabeledModel     = "(未标模型)"
	unlabeledWorkspace = "(未知工作区)"
	unlabeledSession   = "(无会话)"
	remainderSession   = "(其余)"
	maxDrillSessions   = 40
)

// UnlabeledDrillID reports a fallback bucket (no model / workspace / session
// on the event), not a named row. Insights must not call it the "largest".
func UnlabeledDrillID(id string) bool {
	return id == unlabeledModel || id == unlabeledWorkspace || id == unlabeledSession || id == remainderSession
}

type SessionSlice struct {
	Slice
	Source, Vendor, Model, Workspace string
	LastDate                         string
}

type DrillPack struct {
	Models     []Slice
	Workspaces []Slice
	Sessions   []SessionSlice
}

type SessionView struct {
	SliceView
	Source    string `json:"source"`
	Vendor    string `json:"vendor"`
	Model     string `json:"model"`
	Workspace string `json:"workspace"`
	LastDate  string `json:"last_date"`
}

type DrillTablesView struct {
	Models     []SliceView   `json:"models"`
	Workspaces []SliceView   `json:"workspaces"`
	Sessions   []SessionView `json:"sessions"`
}

func ViewSession(s SessionSlice) SessionView {
	return SessionView{
		SliceView: View(s.Slice),
		Source:    s.Source,
		Vendor:    s.Vendor,
		Model:     s.Model,
		Workspace: s.Workspace,
		LastDate:  s.LastDate,
	}
}

func ViewDrill(p DrillPack) DrillTablesView {
	out := DrillTablesView{
		Models:     []SliceView{},
		Workspaces: []SliceView{},
		Sessions:   []SessionView{},
	}
	for _, s := range p.Models {
		out.Models = append(out.Models, View(s))
	}
	for _, s := range p.Workspaces {
		out.Workspaces = append(out.Workspaces, View(s))
	}
	for _, s := range p.Sessions {
		out.Sessions = append(out.Sessions, ViewSession(s))
	}
	return out
}

func buildDrill(events []event.UsageEvent, turns []event.TurnEvent) (DrillPack, map[string]DrillPack, map[string]DrillPack) {
	allModels := map[string]*Slice{}
	allWS := map[string]*Slice{}
	allSess := map[string]*SessionSlice{}
	srcModels := map[string]map[string]*Slice{}
	srcWS := map[string]map[string]*Slice{}
	srcSess := map[string]map[string]*SessionSlice{}
	vendModels := map[string]map[string]*Slice{}
	vendWS := map[string]map[string]*Slice{}
	vendSess := map[string]map[string]*SessionSlice{}

	for _, e := range events {
		ch := price.Event(e)
		modelID := nz(e.Model, unlabeledModel)
		wsID := nz(e.Workspace, unlabeledWorkspace)
		sessID := sessionID(e)
		addSlice(getSlice(allModels, modelID, modelID), e, ch)
		addSlice(getSlice(allWS, wsID, wsID), e, ch)
		addSession(allSess, sessID, e, ch)
		addSlice(getSlice(nested(srcModels, e.Source), modelID, modelID), e, ch)
		addSlice(getSlice(nested(srcWS, e.Source), wsID, wsID), e, ch)
		addSession(nestedSess(srcSess, e.Source), sessID, e, ch)
		addSlice(getSlice(nested(vendModels, e.Vendor), modelID, modelID), e, ch)
		addSlice(getSlice(nested(vendWS, e.Vendor), wsID, wsID), e, ch)
		addSession(nestedSess(vendSess, e.Vendor), sessID, e, ch)
	}

	for _, t := range turns {
		if t.SessionID != "" {
			bumpSessionTurns(allSess, t.SessionID)
			bumpSessionTurns(srcSess[t.Source], t.SessionID)
			if s := allSess[t.SessionID]; s != nil && s.Vendor != "" {
				bumpSessionTurns(vendSess[s.Vendor], t.SessionID)
			}
		}
		if t.Workspace != "" {
			bumpTurns(allWS, t.Workspace)
			bumpTurns(srcWS[t.Source], t.Workspace)
			if s := allSess[t.SessionID]; s != nil && s.Vendor != "" {
				bumpTurns(vendWS[s.Vendor], t.Workspace)
			}
		}
	}

	all := flattenPack(allModels, allWS, allSess)
	bySource := map[string]DrillPack{}
	for id := range srcModels {
		bySource[id] = flattenPack(srcModels[id], srcWS[id], srcSess[id])
	}
	byVendor := map[string]DrillPack{}
	for id := range vendModels {
		byVendor[id] = flattenPack(vendModels[id], vendWS[id], vendSess[id])
	}
	return all, bySource, byVendor
}

func nz(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func sessionID(e event.UsageEvent) string {
	if e.SessionID != "" {
		return e.SessionID
	}
	if e.RequestID != "" {
		return e.RequestID
	}
	return unlabeledSession
}

func nested(m map[string]map[string]*Slice, key string) map[string]*Slice {
	inner := m[key]
	if inner == nil {
		inner = map[string]*Slice{}
		m[key] = inner
	}
	return inner
}

func nestedSess(m map[string]map[string]*SessionSlice, key string) map[string]*SessionSlice {
	inner := m[key]
	if inner == nil {
		inner = map[string]*SessionSlice{}
		m[key] = inner
	}
	return inner
}

func addSession(m map[string]*SessionSlice, id string, e event.UsageEvent, ch price.Charge) {
	s := m[id]
	if s == nil {
		s = &SessionSlice{
			Slice:     Slice{ID: id, Label: id},
			Source:    e.Source,
			Vendor:    e.Vendor,
			Model:     nz(e.Model, unlabeledModel),
			Workspace: nz(e.Workspace, unlabeledWorkspace),
		}
		m[id] = s
	}
	addSlice(&s.Slice, e, ch)
	if e.Model != "" {
		s.Model = e.Model
	}
	if e.Workspace != "" {
		s.Workspace = e.Workspace
	}
	if e.Vendor != "" {
		s.Vendor = e.Vendor
	}
	if !e.Timestamp.IsZero() {
		d := e.Timestamp.In(time.Local).Format("2006-01-02")
		if d > s.LastDate {
			s.LastDate = d
		}
	}
}

func bumpTurns(m map[string]*Slice, id string) {
	if m == nil {
		return
	}
	if s := m[id]; s != nil {
		s.UserTurns++
	}
}

func bumpSessionTurns(m map[string]*SessionSlice, id string) {
	if m == nil {
		return
	}
	if s := m[id]; s != nil {
		s.UserTurns++
	}
}

func flattenPack(models, workspaces map[string]*Slice, sessions map[string]*SessionSlice) DrillPack {
	p := DrillPack{}
	for _, s := range models {
		finishCost(s)
		p.Models = append(p.Models, *s)
	}
	for _, s := range workspaces {
		finishCost(s)
		p.Workspaces = append(p.Workspaces, *s)
	}
	for _, s := range sessions {
		finishCost(&s.Slice)
		p.Sessions = append(p.Sessions, *s)
	}
	sort.Slice(p.Models, func(i, j int) bool { return p.Models[i].Total() > p.Models[j].Total() })
	sort.Slice(p.Workspaces, func(i, j int) bool { return p.Workspaces[i].Total() > p.Workspaces[j].Total() })
	sort.Slice(p.Sessions, func(i, j int) bool { return p.Sessions[i].Total() > p.Sessions[j].Total() })
	p.Sessions = foldSessions(p.Sessions)
	if p.Models == nil {
		p.Models = []Slice{}
	}
	if p.Workspaces == nil {
		p.Workspaces = []Slice{}
	}
	if p.Sessions == nil {
		p.Sessions = []SessionSlice{}
	}
	return p
}

func foldSessions(in []SessionSlice) []SessionSlice {
	if len(in) <= maxDrillSessions {
		return in
	}
	keep := append([]SessionSlice(nil), in[:maxDrillSessions-1]...)
	rest := SessionSlice{Slice: Slice{ID: remainderSession, Label: remainderSession}}
	for _, s := range in[maxDrillSessions-1:] {
		rest.Miss = satAdd(rest.Miss, s.Miss)
		rest.CacheRead = satAdd(rest.CacheRead, s.CacheRead)
		rest.CacheCreate = satAdd(rest.CacheCreate, s.CacheCreate)
		rest.Output = satAdd(rest.Output, s.Output)
		rest.Requests = satAdd(rest.Requests, s.Requests)
		rest.UserTurns = satAdd(rest.UserTurns, s.UserTurns)
		rest.Records = satAdd(rest.Records, s.Records)
		rest.CostMicro = satAdd(rest.CostMicro, s.CostMicro)
		rest.MissCostMicro = satAdd(rest.MissCostMicro, s.MissCostMicro)
		rest.CacheReadCostMicro = satAdd(rest.CacheReadCostMicro, s.CacheReadCostMicro)
		rest.CacheCreateCostMicro = satAdd(rest.CacheCreateCostMicro, s.CacheCreateCostMicro)
		rest.OutputCostMicro = satAdd(rest.OutputCostMicro, s.OutputCostMicro)
		rest.PricedTokens = satAdd(rest.PricedTokens, s.PricedTokens)
		rest.UnpricedTokens = satAdd(rest.UnpricedTokens, s.UnpricedTokens)
	}
	finishCost(&rest.Slice)
	return append(keep, rest)
}
