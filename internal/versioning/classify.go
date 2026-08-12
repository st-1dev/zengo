package versioning

import (
	"fmt"
	"strings"
)

// FieldMapping describes how one field is converted between hub and legacy messages.
type FieldMapping struct {
	// HubField is the mapped hub field name.
	HubField string
	// LegacyField is the mapped legacy field name.
	LegacyField string
	// HubType is the normalized hub field type.
	HubType string
	// LegacyType is the normalized legacy field type.
	LegacyType string
	// Repeated reports whether either side uses repeated semantics.
	Repeated bool
	// Auto reports whether the mapping can be generated automatically.
	Auto bool
	// Reason explains why the mapping is manual or lossy.
	Reason string
}

// MessageConversion describes the conversion policy for one message pair.
type MessageConversion struct {
	// HubMessage is the hub message name.
	HubMessage string
	// LegacyMessage is the legacy message name.
	LegacyMessage string
	// Mode is the conversion mode, for example AUTO or MANUAL.
	Mode string // AUTO or MANUAL
	// Reason explains why the conversion needs manual work.
	Reason string
	// Fields describes field-level mappings between the message versions.
	Fields []FieldMapping
}

// RPCConversion describes the conversion policy for one RPC pair.
type RPCConversion struct {
	// HubRPC is the hub RPC signature.
	HubRPC RPC
	// LegacyRPC is the legacy RPC signature.
	LegacyRPC RPC
	// Mode is the conversion mode, for example AUTO or MANUAL.
	Mode string
	// Reason explains why the conversion needs manual work.
	Reason string
}

// ConversionPlan is the compatibility plan for one legacy version.
type ConversionPlan struct {
	// Version is the legacy API version being planned.
	Version string
	// Messages contains message conversion decisions.
	Messages []MessageConversion
	// RPCs contains RPC conversion decisions.
	RPCs []RPCConversion
	// Errors lists required manual implementations before generation can proceed.
	Errors []ManualRequired
}

// ManualRequired describes one missing manual implementation required by BuildPlan.
type ManualRequired struct {
	// Version is the legacy API version that requires manual code.
	Version string
	// HubMessage is the hub-side symbol involved in the incompatibility.
	HubMessage string
	// LegacyMessage is the legacy-side symbol involved in the incompatibility.
	LegacyMessage string
	// Reason explains why manual code is required.
	Reason string
	// ManualPath is the expected service-local file path for the manual implementation.
	ManualPath string
}

// BuildPlan compares hub and legacy schemas and reports what compatibility code is required.
func BuildPlan(version string, hub, legacy *Schema) ConversionPlan {
	plan := ConversionPlan{Version: version}

	for legacyName, legacyMsg := range legacy.Messages {
		hubMsg, ok := hub.Messages[legacyName]
		if !ok {
			plan.Errors = append(plan.Errors, ManualRequired{
				Version:       version,
				LegacyMessage: legacyName,
				Reason:        "message removed from hub",
				ManualPath:    manualPath(version, legacyName),
			})
			continue
		}
		conv := compareMessages(hubMsg, legacyMsg)
		if conv.Mode == "MANUAL" {
			plan.Errors = append(plan.Errors, ManualRequired{
				Version:       version,
				HubMessage:    hubMsg.Name,
				LegacyMessage: legacyMsg.Name,
				Reason:        conv.Reason,
				ManualPath:    manualPath(version, legacyName),
			})
		}
		plan.Messages = append(plan.Messages, conv)
	}

	plan.RPCs = compareRPCs(hub, legacy)
	for _, rpc := range plan.RPCs {
		switch rpc.Mode {
		case "MANUAL":
			plan.Errors = append(plan.Errors, ManualRequired{
				Version:    version,
				HubMessage: rpc.LegacyRPC.Name,
				Reason:     rpc.Reason,
				ManualPath: manualPath(version, rpc.LegacyRPC.Service+"_"+rpc.LegacyRPC.Name),
			})
		case "UNIMPLEMENTED":
			plan.Errors = append(plan.Errors, ManualRequired{
				Version:    version,
				HubMessage: rpc.LegacyRPC.Name,
				Reason:     rpc.Reason,
				ManualPath: manualPath(version, rpc.LegacyRPC.Service+"_"+rpc.LegacyRPC.Name+"_removed"),
			})
		}
	}
	return plan
}

func compareMessages(hub, legacy Message) MessageConversion {
	conv := MessageConversion{
		HubMessage:    hub.Name,
		LegacyMessage: legacy.Name,
		Mode:          "AUTO",
	}

	legacyByName := map[string]Field{}
	for _, f := range legacy.Fields {
		legacyByName[f.Name] = f
	}
	hubByName := map[string]Field{}
	for _, f := range hub.Fields {
		hubByName[f.Name] = f
	}

	for _, lf := range legacy.Fields {
		hf, ok := hubByName[lf.Name]
		if !ok {
			for _, h := range hub.Fields {
				if h.LegacyName == lf.Name {
					hf = h
					ok = true
					break
				}
			}
		}
		if !ok {
			// field removed in hub — legacy->hub uses zero value
			conv.Fields = append(conv.Fields, FieldMapping{
				LegacyField: lf.Name,
				LegacyType:  lf.Type,
				Auto:        true,
			})
			continue
		}
		m := FieldMapping{
			HubField:    hf.Name,
			LegacyField: lf.Name,
			HubType:     hf.Type,
			LegacyType:  lf.Type,
			Repeated:    hf.Repeated || lf.Repeated,
		}
		if hf.Type != lf.Type || hf.Repeated != lf.Repeated {
			m.Auto = false
			conv.Mode = "MANUAL"
			conv.Reason = fmt.Sprintf("field %q type changed (%s -> %s)", lf.Name, lf.Type, hf.Type)
		} else {
			m.Auto = true
		}
		conv.Fields = append(conv.Fields, m)
	}

	for _, hf := range hub.Fields {
		_, ok := legacyByName[hf.Name]
		if !ok {
			// field added in hub — dropped on hub->legacy
			conv.Fields = append(conv.Fields, FieldMapping{
				HubField: hf.Name,
				HubType:  hf.Type,
				Auto:     true,
			})
		}
	}

	return conv
}

func compareRPCs(hub, legacy *Schema) []RPCConversion {
	var out []RPCConversion
	legacyRPCs := map[string]RPC{}
	for _, svc := range legacy.Services {
		for _, rpc := range svc.RPCs {
			legacyRPCs[rpc.Name] = rpc
		}
	}
	hubRPCs := map[string]RPC{}
	for _, svc := range hub.Services {
		for _, rpc := range svc.RPCs {
			hubRPCs[rpc.Name] = rpc
		}
	}

	for name, lr := range legacyRPCs {
		hr, ok := hubRPCs[name]
		if !ok {
			out = append(out, RPCConversion{
				LegacyRPC: lr,
				Mode:      "UNIMPLEMENTED",
				Reason:    "rpc removed from hub",
			})
			continue
		}
		mode := "AUTO"
		reason := ""
		if hr.RequestType != lr.RequestType || hr.ResponseType != lr.ResponseType {
			mode = "MANUAL"
			reason = fmt.Sprintf("rpc %q request/response types differ", name)
		}
		out = append(out, RPCConversion{
			HubRPC:    hr,
			LegacyRPC: lr,
			Mode:      mode,
			Reason:    reason,
		})
	}
	return out
}

func manualPath(version, name string) string {
	safe := strings.ToLower(name)
	safe = strings.ReplaceAll(safe, ".", "_")
	return fmt.Sprintf("internal/convert/%s/%s.go", version, safe)
}
