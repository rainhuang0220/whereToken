package metric

import (
	"sort"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

const (
	unlabeledModel     = "(未标模型)"
	unlabeledWorkspace = "(未知工作区)"
	unlabeledSession   = "(无会话)"
)

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
		modelID := nz(e.Model, unlabeledModel)
		wsID := nz(e.Workspace, unlabeledWorkspace)
		sessID := sessionID(e)
		addSlice(getSlice(allModels, modelID, modelID), e)
		addSlice(getSlice(allWS, wsID, wsID), e)
		addSession(allSess, sessID, e)
		addSlice(getSlice(nested(srcModels, e.Source), modelID, modelID), e)
		addSlice(getSlice(nested(srcWS, e.Source), wsID, wsID), e)
		addSession(nestedSess(srcSess, e.Source), sessID, e)
		addSlice(getSlice(nested(vendModels, e.Vendor), modelID, modelID), e)
		addSlice(getSlice(nested(vendWS, e.Vendor), wsID, wsID), e)
		addSession(nestedSess(vendSess, e.Vendor), sessID, e)
	}

	for _, t := range turns {
		if t.SessionID != "" {
			bumpSessionTurns(allSess, t.SessionID)
			bumpSessionTurns(srcSess[t.Source], t.SessionID)
		}
		if t.Workspace != "" {
			bumpTurns(allWS, t.Workspace)
			bumpTurns(srcWS[t.Source], t.Workspace)
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

func addSession(m map[string]*SessionSlice, id string, e event.UsageEvent) {
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
	addSlice(&s.Slice, e)
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
