// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package apk

import (
	"errors"
	"slices"
	"strconv"
	"strings"
)

var errNoPackage = errors.New("manifest has no package name")

// Manifest is the typed content of AndroidManifest.xml.
type Manifest struct {
	Package          string `json:"package"`
	VersionCode      int64  `json:"version_code"`
	VersionName      string `json:"version_name"`
	MinSDK           int    `json:"min_sdk"`
	TargetSDK        int    `json:"target_sdk"`
	MaxSDK           int    `json:"max_sdk,omitempty"`
	CompileSDK       int    `json:"compile_sdk,omitempty"`
	CompileSDKName   string `json:"compile_sdk_codename,omitempty"`
	SharedUserID     string `json:"shared_user_id,omitempty"`
	InstallLocation  string `json:"install_location,omitempty"`
	IsSplit          bool   `json:"is_split,omitempty"`
	Split            string `json:"split,omitempty"`
	RequiredSplitIDs string `json:"required_split_types,omitempty"`

	Application AppFlags     `json:"application"`
	Permissions []Permission `json:"permissions"`
	Declared    []Permission `json:"declared_permissions,omitempty"`
	Features    []Feature    `json:"features"`
	Components  []Component  `json:"components"`
	Queries     []string     `json:"queries,omitempty"`
	Libraries   []string     `json:"uses_libraries,omitempty"`

	// Resource references, resolved separately against resources.arsc.
	labelRef      Attr
	iconRef       Attr
	roundIconRef  Attr
	launcherLabel Attr
	launcherIcon  Attr
}

// AppFlags are the security-relevant <application> attributes.
type AppFlags struct {
	Debuggable            bool   `json:"debuggable"`
	TestOnly              bool   `json:"test_only,omitempty"`
	AllowBackup           *bool  `json:"allow_backup,omitempty"`
	UsesCleartextTraffic  *bool  `json:"uses_cleartext_traffic,omitempty"`
	NetworkSecurityConfig string `json:"network_security_config,omitempty"`
	ExtractNativeLibs     *bool  `json:"extract_native_libs,omitempty"`
	HasCode               *bool  `json:"has_code,omitempty"`
	Name                  string `json:"name,omitempty"`
	AppComponentFactory   string `json:"app_component_factory,omitempty"`
}

// Permission is a uses-permission (or a declared <permission>).
type Permission struct {
	Name            string `json:"name"`
	MaxSDK          int    `json:"max_sdk,omitempty"`
	ProtectionLevel string `json:"protection_level,omitempty"`
	SDK23           bool   `json:"sdk23,omitempty"`
}

// Feature is a uses-feature.
type Feature struct {
	Name        string `json:"name,omitempty"`
	Required    bool   `json:"required"`
	GLESVersion string `json:"gles_version,omitempty"`
}

// Component is an activity, service, receiver or provider.
type Component struct {
	Kind        string         `json:"kind"`
	Name        string         `json:"name"`
	Exported    *bool          `json:"exported,omitempty"`
	Enabled     *bool          `json:"enabled,omitempty"`
	Permission  string         `json:"permission,omitempty"`
	Authorities string         `json:"authorities,omitempty"`
	Launcher    bool           `json:"launcher,omitempty"`
	Filters     []IntentFilter `json:"intent_filters,omitempty"`
}

// IntentFilter is one <intent-filter>.
type IntentFilter struct {
	Actions    []string `json:"actions,omitempty"`
	Categories []string `json:"categories,omitempty"`
	Data       []string `json:"data,omitempty"`
}

func optBool(e *Element, name string) *bool {
	if b, ok := e.boolAttr(name); ok {
		return &b
	}

	return nil
}

// manifestFrom maps the decoded manifest tree to a Manifest.
func manifestFrom(root *Element) (*Manifest, error) {
	m := &Manifest{Package: root.str("package")}
	if m.Package == "" {
		// "package" has no resource id; packers sometimes give it a namespace.
		for _, a := range root.Attrs {
			if a.Name == "package" {
				m.Package = a.Raw
			}
		}
	}

	if m.Package == "" {
		return nil, errNoPackage
	}

	code, _ := root.intAttr("versionCode")
	major, _ := root.intAttr("versionCodeMajor")
	m.VersionCode = major<<32 | code&0xffffffff
	m.VersionName = root.str("versionName")
	m.SharedUserID = root.str("sharedUserId")
	m.Split = root.str("split")
	m.IsSplit = m.Split != ""

	if n, ok := root.intAttr("compileSdkVersion"); ok {
		m.CompileSDK = int(n)
	}

	m.CompileSDKName = root.str("compileSdkVersionCodename")

	if n, ok := root.intAttr("installLocation"); ok {
		m.InstallLocation = map[int64]string{0: "auto", 1: "internalOnly", 2: "preferExternal"}[n]
	}

	m.MinSDK = 1

	for _, c := range root.Children {
		m.child(c)
	}

	return m, nil
}

func (m *Manifest) child(c *Element) {
	switch c.Name {
	case "uses-sdk":
		m.usesSDK(c)
	case "uses-permission", "uses-permission-sdk-23", "uses-permission-sdk-m":
		p := Permission{Name: c.str("name"), SDK23: c.Name != "uses-permission"}
		if n, ok := c.intAttr("maxSdkVersion"); ok {
			p.MaxSDK = int(n)
		}

		if p.Name != "" {
			m.Permissions = append(m.Permissions, p)
		}
	case "permission":
		p := Permission{Name: c.str("name")}
		if n, ok := c.intAttr("protectionLevel"); ok {
			p.ProtectionLevel = protectionLevel(n)
		}

		m.Declared = append(m.Declared, p)
	case "uses-feature":
		f := Feature{Name: c.str("name"), Required: true}
		if b, ok := c.boolAttr("required"); ok {
			f.Required = b
		}

		if n, ok := c.intAttr("glEsVersion"); ok && f.Name == "" {
			f.GLESVersion = glesVersion(n)
		}

		m.Features = append(m.Features, f)
	case "queries":
		for _, q := range c.Children {
			switch q.Name {
			case "package":
				m.Queries = append(m.Queries, q.str("name"))
			case "provider":
				m.Queries = append(m.Queries, "provider:"+q.str("authorities"))
			case "intent":
				m.Queries = append(m.Queries, "intent:"+strings.Join(intentFilter(q).Actions, ","))
			}
		}
	case "application":
		m.application(c)
	}
}

func (m *Manifest) usesSDK(c *Element) {
	if n, ok := c.intAttr("minSdkVersion"); ok {
		m.MinSDK = int(n)
	}

	if n, ok := c.intAttr("targetSdkVersion"); ok {
		m.TargetSDK = int(n)
	}

	if n, ok := c.intAttr("maxSdkVersion"); ok {
		m.MaxSDK = int(n)
	}
}

func (m *Manifest) application(app *Element) {
	m.Application = AppFlags{
		AllowBackup:           optBool(app, "allowBackup"),
		UsesCleartextTraffic:  optBool(app, "usesCleartextTraffic"),
		ExtractNativeLibs:     optBool(app, "extractNativeLibs"),
		HasCode:               optBool(app, "hasCode"),
		NetworkSecurityConfig: app.str("networkSecurityConfig"),
		Name:                  app.str("name"),
	}
	m.Application.Debuggable, _ = app.boolAttr("debuggable")
	m.Application.TestOnly, _ = app.boolAttr("testOnly")
	m.labelRef, _ = app.attr("label")
	m.iconRef, _ = app.attr("icon")
	m.roundIconRef, _ = app.attr("roundIcon")

	for _, c := range app.Children {
		switch c.Name {
		case "activity", "activity-alias", "service", "receiver", "provider":
			comp := m.component(c)
			m.Components = append(m.Components, comp)
		case "uses-library", "uses-native-library":
			m.Libraries = append(m.Libraries, c.str("name"))
		}
	}

	if m.TargetSDK == 0 {
		m.TargetSDK = m.MinSDK
	}
}

func (m *Manifest) component(c *Element) Component {
	comp := Component{
		Kind:        c.Name,
		Name:        expandClass(m.Package, c.str("name")),
		Exported:    optBool(c, "exported"),
		Enabled:     optBool(c, "enabled"),
		Permission:  c.str("permission"),
		Authorities: c.str("authorities"),
	}

	c.each("intent-filter", func(f *Element) {
		filter := intentFilter(f)
		comp.Filters = append(comp.Filters, filter)

		if (c.Name == "activity" || c.Name == "activity-alias") &&
			slices.Contains(filter.Actions, "android.intent.action.MAIN") &&
			slices.Contains(filter.Categories, "android.intent.category.LAUNCHER") {
			comp.Launcher = true
		}
	})

	// Disabled launcher aliases (icon disguises) are not what the user sees.
	if comp.Launcher && m.launcherLabel.Name == "" && (comp.Enabled == nil || *comp.Enabled) {
		m.launcherLabel, _ = c.attr("label")
		m.launcherIcon, _ = c.attr("icon")
	}

	return comp
}

func intentFilter(f *Element) IntentFilter {
	var out IntentFilter

	for _, c := range f.Children {
		switch c.Name {
		case "action":
			out.Actions = append(out.Actions, c.str("name"))
		case "category":
			out.Categories = append(out.Categories, c.str("name"))
		case "data":
			out.Data = append(out.Data, dataSpec(c))
		}
	}

	return out
}

func dataSpec(c *Element) string {
	var b strings.Builder

	if s := c.str("scheme"); s != "" {
		b.WriteString(s + "://")
	}

	b.WriteString(c.str("host"))

	if p := c.str("port"); p != "" {
		b.WriteString(":" + p)
	}

	for _, k := range []string{"path", "pathPrefix", "pathPattern"} {
		if p := c.str(k); p != "" {
			b.WriteString(p)
		}
	}

	if mt := c.str("mimeType"); mt != "" {
		if b.Len() > 0 {
			b.WriteString(" ")
		}

		b.WriteString(mt)
	}

	return b.String()
}

// expandClass turns ".Foo" into "pkg.Foo".
func expandClass(pkg, name string) string {
	if strings.HasPrefix(name, ".") {
		return pkg + name
	}

	return name
}

func protectionLevel(n int64) string {
	base := map[int64]string{0: "normal", 1: "dangerous", 2: "signature", 3: "signatureOrSystem", 4: "internal"}[n&0xf]
	if base == "" {
		base = "normal"
	}

	return base
}

func glesVersion(n int64) string {
	return strconv.FormatInt(n>>16, 10) + "." + strconv.FormatInt(n&0xffff, 10)
}
