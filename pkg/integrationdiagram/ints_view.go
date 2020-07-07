package integrationdiagram

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/anz-bank/sysl/pkg/cmdutils"
	"github.com/anz-bank/sysl/pkg/sysl"
	"github.com/anz-bank/sysl/pkg/syslutil"
)

const AppfmtDefault = "%(appname)"
const ArrowColorNone = "none"
const PumlHeader = `''''''''''''''''''''''''''''''''''''''''''
''                                      ''
''  AUTOGENERATED CODE -- DO NOT EDIT!  ''
''                                      ''
''''''''''''''''''''''''''''''''''''''''''

`
const ComponentStart = `hide stereotype
scale max 16384 height
skinparam component {
  BackgroundColor FloralWhite
  BorderColor Black
  ArrowColor Crimson
`
const StateStart = `left to right direction
scale max 16384 height
hide empty description
skinparam state {
  BackgroundColor FloralWhite
  BorderColor Black
  ArrowColor Crimson
`

type IntsParam struct {
	Apps         []string
	DrawableApps map[string]struct{}
	Integrations []AppDependency
	App          *sysl.Application
	Endpt        *sysl.Endpoint
}

type Args struct {
	Title     string
	Project   string
	Clustered bool
	Epa       bool
}

type IntsDiagramVisitor struct {
	Mod           *sysl.Module
	StringBuilder *strings.Builder
	DrawableApps  map[string]struct{}
	Symbols       map[string]*cmdutils.Var
	TopSymbols    map[string]*_topVar
	Project       string
}

type _topVar struct {
	TopLabel string
	TopAlias string
}

type AppPair struct {
	Self   string
	Target string
}

type ViewParams struct {
	RestrictBy         string
	EndptAttrs         map[string]*sysl.Attribute
	HighLightColor     string
	ArrowColor         string
	IndirectArrowColor string
	DiagramTitle       string
}

func MakeIntsDiagramVisitor(
	mod *sysl.Module, stringBuilder *strings.Builder,
	drawableApps map[string]struct{}, project string,
) *IntsDiagramVisitor {
	return &IntsDiagramVisitor{
		Mod:           mod,
		StringBuilder: stringBuilder,
		DrawableApps:  drawableApps,
		Symbols:       map[string]*cmdutils.Var{},
		TopSymbols:    map[string]*_topVar{},
		Project:       project,
	}
}

func (v *IntsDiagramVisitor) VarManagerForComponent(appName string, nameMap map[string]string) string {
	if key, ok := nameMap[appName]; ok {
		appName = key
	}
	if s, ok := v.Symbols[appName]; ok {
		return s.Alias
	}

	fp := cmdutils.MakeFormatParser(getAppfmtAttrOrDefault(v.Mod.Apps[v.Project]))
	attrs := cmdutils.GetApplicationAttrs(v.Mod, appName)
	controls := cmdutils.GetSortedISOCtrlStr(attrs)

	label := fp.LabelApp(appName, controls, attrs)
	i := len(v.Symbols)
	alias := fmt.Sprintf("_%d", i)
	s := &cmdutils.Var{
		Label: label,
		Alias: alias,
	}
	v.Symbols[appName] = s
	if _, ok := v.DrawableApps[appName]; ok {
		fmt.Fprintf(v.StringBuilder, "[%s] as %s <<highlight>>\n", label, alias)
	} else {
		fmt.Fprintf(v.StringBuilder, "[%s] as %s\n", label, alias)
	}
	return s.Alias
}

func (v *IntsDiagramVisitor) VarManagerForTopState(appName string) string {
	var alias, label string
	if ts, ok := v.TopSymbols[appName]; ok {
		return ts.TopAlias
	}
	i := len(v.TopSymbols)
	alias = fmt.Sprintf("_%d", i)

	fp := cmdutils.MakeFormatParser(getAppfmtAttrOrDefault(v.Mod.Apps[v.Project]))
	attrs := cmdutils.GetApplicationAttrs(v.Mod, appName)
	controls := cmdutils.GetSortedISOCtrlStr(attrs)
	label = fp.LabelApp(appName, controls, attrs)
	ts := &_topVar{
		TopLabel: label,
		TopAlias: alias,
	}
	v.TopSymbols[appName] = ts
	if _, ok := v.DrawableApps[appName]; ok {
		fmt.Fprintf(v.StringBuilder, "state \"%s\" as X%s <<highlight>> {\n", label, alias)
	} else {
		fmt.Fprintf(v.StringBuilder, "state \"%s\" as X%s {\n", label, alias)
	}

	return ts.TopAlias
}

func (v *IntsDiagramVisitor) VarManagerForEPA(name string) string {
	var appName, alias, label string
	attrs := map[string]string{}

	appName = strings.Split(name, " : ")[0]
	epName := strings.Split(name, " : ")[1]

	if s, ok := v.Symbols[name]; ok {
		return s.Alias
	}
	i := len(v.Symbols)
	alias = fmt.Sprintf("_%d", i)

	if v.Mod.Apps[appName].Endpoints[epName] != nil {
		for k, v := range v.Mod.Apps[appName].Endpoints[epName].Attrs {
			attrs["@"+k] = v.GetS()
		}
	}
	attrs["appname"] = epName
	fp := cmdutils.MakeFormatParser(getAppfmtAttrOrDefault(v.Mod.Apps[v.Project]))
	label = fp.Parse(attrs)

	s := &cmdutils.Var{
		Label: label,
		Alias: alias,
	}
	v.Symbols[name] = s

	if _, ok := v.DrawableApps[appName]; ok {
		fmt.Fprintf(v.StringBuilder, "  state \"%s\" as %s <<highlight>>\n", label, alias)
	} else {
		fmt.Fprintf(v.StringBuilder, "  state \"%s\" as %s\n", label, alias)
	}
	return s.Alias
}

func (v *IntsDiagramVisitor) BuildClusterForEPAView(deps []AppDependency, restrictBy string) {
	clusters := map[string][]string{}
	for _, dep := range deps {
		appA := dep.Self.Name
		appB := dep.Target.Name
		epA := dep.Self.Endpoint
		epB := dep.Target.Endpoint
		_, okA := v.Mod.Apps[appA].Endpoints[epA].Attrs[restrictBy]
		_, okB := v.Mod.Apps[appB].Endpoints[epB].Attrs[restrictBy]
		if _, ok := v.Mod.Apps[appA].Attrs[restrictBy]; !ok && restrictBy != "" {
			if _, ok := v.Mod.Apps[appB].Attrs[restrictBy]; !ok {
				continue
			}
		}
		if !okA && restrictBy != "" && !okB {
			continue
		}
		clusters[appA] = append(clusters[appA], epA)
		if appA != appB && !v.Mod.Apps[appA].Endpoints[epA].IsPubsub {
			clusters[appA] = append(clusters[appA], epB+" client")
		}
		clusters[appB] = append(clusters[appB], epB)
	}

	keys := []string{}
	for k := range clusters {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		v.VarManagerForTopState(k)
		strSet := syslutil.MakeStrSet(clusters[k]...)
		for _, m := range strSet.ToSortedSlice() {
			v.VarManagerForEPA(k + " : " + m)
		}
		v.StringBuilder.WriteString("}\n")
	}
}

func (v *IntsDiagramVisitor) BuildClusterForIntsView(apps []string) map[string]string {
	nameMap := map[string]string{}
	clusters := map[string][]string{}
	for _, v := range apps {
		cluster := strings.Split(v, " :: ")
		if len(cluster) > 1 {
			clusters[cluster[0]] = append(clusters[cluster[0]], v)
		}
	}

	for k, v := range clusters {
		if len(v) <= 1 {
			delete(clusters, k)
		}
		for _, s := range v {
			nameMap[s] = strings.Split(s, " :: ")[1]
		}
	}

	for k, apps := range clusters {
		fmt.Fprintf(v.StringBuilder, "package \"%s\" {\n", k)
		for _, n := range apps {
			v.VarManagerForComponent(n, nameMap)
		}
		v.StringBuilder.WriteString("}\n")
	}

	return nameMap
}

func (v *IntsDiagramVisitor) GenerateEPAView(viewParams ViewParams, params *IntsParam) string {
	v.StringBuilder.WriteString("@startuml\n")
	if viewParams.DiagramTitle != "" {
		fmt.Fprintf(v.StringBuilder, "title %s\n", viewParams.DiagramTitle)
	}
	v.StringBuilder.WriteString(StateStart)
	if viewParams.HighLightColor != "" {
		fmt.Fprintf(v.StringBuilder, "  BackgroundColor<<highlight>> %s\n", viewParams.HighLightColor)
	}
	if viewParams.ArrowColor != "" {
		fmt.Fprintf(v.StringBuilder, "  ArrowColor %s\n", viewParams.ArrowColor)
	}

	if viewParams.IndirectArrowColor != "" && viewParams.IndirectArrowColor != ArrowColorNone {
		fmt.Fprintf(v.StringBuilder, "  ArrowColor<<indirect>> %s\n", viewParams.IndirectArrowColor)
	}
	v.StringBuilder.WriteString("}\n")
	v.BuildClusterForEPAView(params.Integrations, viewParams.RestrictBy)
	var processed []string
	for _, dep := range params.Integrations {
		appA := dep.Self.Name
		appB := dep.Target.Name
		epA := dep.Self.Endpoint
		epB := dep.Target.Endpoint
		_, restrictByAppA := v.Mod.Apps[appA].Attrs[viewParams.RestrictBy]
		_, restrictByAppB := v.Mod.Apps[appB].Attrs[viewParams.RestrictBy]
		_, restrictByEpA := v.Mod.Apps[appA].Endpoints[epA].Attrs[viewParams.RestrictBy]
		_, restrictByEpB := v.Mod.Apps[appB].Endpoints[epB].Attrs[viewParams.RestrictBy]
		if viewParams.RestrictBy != "" && !(restrictByAppA || restrictByAppB) {
			continue
		}
		if viewParams.RestrictBy != "" && !(restrictByEpA || restrictByEpB) {
			continue
		}
		matchApp := appB
		matchEp := epB
		label := ""
		needsInt := appA != matchApp

		pubSubSrcPtrns := syslutil.MakeStrSetFromAttr("patterns", v.Mod.Apps[appA].Endpoints[epA].Attrs)
		attrs := map[string]string{}
		for k, v := range dep.Statement.GetAttrs() {
			attrs["@"+k] = v.GetS()
		}
		var ptrns string
		var targetPatterns syslutil.StrSet
		var srcPtrns syslutil.StrSet
		if v.Mod.Apps[matchApp].Endpoints[matchEp].Attrs["patterns"] != nil {
			targetPatterns = syslutil.MakeStrSetFromAttr("patterns", v.Mod.Apps[matchApp].Endpoints[matchEp].Attrs)
		} else {
			targetPatterns = syslutil.MakeStrSetFromAttr("patterns", v.Mod.Apps[matchApp].Attrs)
		}
		if dep.Statement.GetAttrs()["patterns"] != nil {
			srcPtrns = syslutil.MakeStrSetFromAttr("patterns", dep.Statement.GetAttrs())
		} else {
			srcPtrns = pubSubSrcPtrns
		}
		ptrns = strings.Join(srcPtrns.ToSlice(), ", ") + " → " + strings.Join(targetPatterns.ToSlice(), ", ")
		attrs["patterns"] = ptrns
		if needsInt {
			attrs["needs_int"] = strconv.FormatBool(needsInt)
		}
		fp := cmdutils.MakeFormatParser(params.App.Attrs["epfmt"].GetS())
		label = fp.Parse(attrs)
		flow := strings.Join([]string{appA, epB, appB, epB}, ".")
		isPubSub := v.Mod.Apps[appA].Endpoints[epA].GetIsPubsub()
		epBClient := epB + " client"
		if appA != appB {
			if label != "" {
				label = " : " + label
			}
			if isPubSub {
				fmt.Fprintf(
					v.StringBuilder,
					"%s -%s> %s%s\n", v.VarManagerForEPA(appA+" : "+epA),
					"[#blue]",
					v.VarManagerForEPA(appB+" : "+epB),
					label,
				)
			} else {
				color := ""
				if viewParams.IndirectArrowColor == "" {
					color = "[#silver]-"
				} else {
					color = "[#" + viewParams.IndirectArrowColor + "]-"
				}
				fmt.Fprintf(
					v.StringBuilder,
					"%s -%s> %s\n",
					v.VarManagerForEPA(appA+" : "+epA),
					color,
					v.VarManagerForEPA(appA+" : "+epBClient),
				)
				if !StringInSlice(flow, processed) {
					fmt.Fprintf(
						v.StringBuilder,
						"%s -%s> %s%s\n",
						v.VarManagerForEPA(appA+" : "+epBClient),
						"[#black]",
						v.VarManagerForEPA(appB+" : "+epB),
						label,
					)
					processed = append(processed, flow)
				}
			}
		} else {
			color := ""
			if viewParams.IndirectArrowColor == "" {
				color = "[#silver]-"
			} else {
				color = "[#" + viewParams.IndirectArrowColor + "]-"
			}
			fmt.Fprintf(
				v.StringBuilder,
				"%s -%s> %s%s\n",
				v.VarManagerForEPA(appA+" : "+epA),
				color,
				v.VarManagerForEPA(appB+" : "+epB),
				label,
			)
		}
	}
	v.StringBuilder.WriteString("@enduml")
	return v.StringBuilder.String()
}

func (v *IntsDiagramVisitor) DrawIntsView(viewParams ViewParams, params *IntsParam, nameMap map[string]string) {
	callsDrawn := map[AppPair]struct{}{}
	if viewParams.EndptAttrs["view"].GetS() == "system" {
		v.DrawSystemView(viewParams, params, nameMap)
	} else {
		for _, dep := range params.Integrations {
			appA := dep.Self.Name
			appB := dep.Target.Name
			if appA == appB {
				continue
			}
			appPair := AppPair{
				Self:   appA,
				Target: appB,
			}
			var direct []string
			if _, ok := params.DrawableApps[appA]; ok {
				direct = append(direct, appA)
			}
			if _, ok := params.DrawableApps[appB]; ok {
				direct = append(direct, appB)
			}
			if _, ok := callsDrawn[appPair]; !ok {
				if len(direct) > 0 || direct != nil || viewParams.IndirectArrowColor != ArrowColorNone {
					indirect := ""
					if len(direct) == 0 {
						indirect = " <<indirect>>"
					}
					fmt.Fprintf(
						v.StringBuilder,
						"%s --> %s%s\n",
						v.VarManagerForComponent(appA, nameMap),
						v.VarManagerForComponent(appB, nameMap),
						indirect,
					)
					callsDrawn[appPair] = struct{}{}
				}
			}
		}
		for _, app := range params.Apps {
			for _, mixin := range v.Mod.Apps[app].GetMixin2() {
				mixinName := strings.Join(mixin.Name.Part, " :: ")
				fmt.Fprintf(
					v.StringBuilder,
					"%s <|.. %s\n",
					v.VarManagerForComponent(mixinName, nameMap),
					v.VarManagerForComponent(app, nameMap),
				)
			}
		}
	}
}

func (v *IntsDiagramVisitor) DrawSystemView(viewParams ViewParams, params *IntsParam, nameMap map[string]string) {
	callsDrawn := map[AppPair]struct{}{}
	for _, dep := range params.Integrations {
		appA := dep.Self.Name
		appB := dep.Target.Name
		if appA == appB {
			continue
		}
		appPair := AppPair{
			Self:   appA,
			Target: appB,
		}
		var direct []string
		if _, ok := params.DrawableApps[appA]; ok {
			direct = append(direct, appA)
		}
		if _, ok := params.DrawableApps[appB]; ok {
			direct = append(direct, appB)
		}
		appA = strings.Split(appA, " :: ")[0]
		appB = strings.Split(appB, " :: ")[0]
		if _, ok := callsDrawn[appPair]; !ok {
			if len(direct) > 0 || direct != nil || viewParams.IndirectArrowColor != ArrowColorNone {
				indirect := ""
				if len(direct) == 0 {
					indirect = " <<indirect>>"
				}
				fmt.Fprintf(
					v.StringBuilder,
					"%s --> %s%s\n",
					v.VarManagerForComponent(appA, nameMap),
					v.VarManagerForComponent(appB, nameMap),
					indirect,
				)
				callsDrawn[appPair] = struct{}{}
			}
		}
	}
}

func (v *IntsDiagramVisitor) GenerateIntsView(args *Args, viewParams ViewParams, params *IntsParam) string {
	v.StringBuilder.WriteString("@startuml\n")
	if viewParams.DiagramTitle != "" {
		fmt.Fprintf(v.StringBuilder, "title %s\n", viewParams.DiagramTitle)
	}
	v.StringBuilder.WriteString(ComponentStart)
	if viewParams.HighLightColor != "" {
		fmt.Fprintf(v.StringBuilder, "  BackgroundColor<<highlight>> %s\n", viewParams.HighLightColor)
	}
	if viewParams.ArrowColor != "" {
		fmt.Fprintf(v.StringBuilder, "  ArrowColor %s\n", viewParams.ArrowColor)
	}

	if viewParams.IndirectArrowColor != "" && viewParams.IndirectArrowColor != ArrowColorNone {
		fmt.Fprintf(v.StringBuilder, "  ArrowColor<<indirect>> %s\n", viewParams.IndirectArrowColor)
	}
	v.StringBuilder.WriteString("}\n")
	nameMap := map[string]string{}
	if args.Clustered || viewParams.EndptAttrs["view"].GetS() == "clustered" {
		nameMap = v.BuildClusterForIntsView(params.Apps)
	}
	v.DrawIntsView(viewParams, params, nameMap)
	v.StringBuilder.WriteString("@enduml")
	return v.StringBuilder.String()
}

func GenerateView(args *Args, params *IntsParam, mod *sysl.Module) string {
	var stringBuilder strings.Builder
	var titleParser *cmdutils.FormatParser
	v := MakeIntsDiagramVisitor(mod, &stringBuilder, params.DrawableApps, args.Project)
	restrictBy := ""
	if params.Endpt.Attrs["restrict_by"] != nil {
		restrictBy = params.Endpt.Attrs["restrict_by"].GetS()
	}

	appAttrs := params.App.Attrs
	endptAttrs := params.Endpt.Attrs
	highLightColor := appAttrs["highlight_color"].GetS()
	arrowColor := appAttrs["arrow_color"].GetS()
	indirectArrowColor := appAttrs["indirect_arrow_color"].GetS()

	attrs := map[string]string{
		"epname":     params.Endpt.Name,
		"eplongname": params.Endpt.LongName,
	}
	title := args.Title
	if appAttrs["title"].GetS() != "" {
		title = appAttrs["title"].GetS()
	}
	titleParser = cmdutils.MakeFormatParser(title)
	diagramTitle := titleParser.Parse(attrs)

	viewParams := &ViewParams{
		RestrictBy:         restrictBy,
		EndptAttrs:         endptAttrs,
		HighLightColor:     highLightColor,
		ArrowColor:         arrowColor,
		IndirectArrowColor: indirectArrowColor,
		DiagramTitle:       diagramTitle,
	}
	v.StringBuilder.WriteString(PumlHeader)

	if args.Epa || endptAttrs["view"].GetS() == "epa" {
		return v.GenerateEPAView(*viewParams, params)
	}
	return v.GenerateIntsView(args, *viewParams, params)
}

func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// getAppfmtAttrOrDefault returns the appfmt attribute value of project if specified or its default.
func getAppfmtAttrOrDefault(project *sysl.Application) string {
	a := project.GetAttrs()["appfmt"].GetS()
	if a != "" {
		return a
	}
	return AppfmtDefault
}
