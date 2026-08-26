# mTLS Client-Auth Validation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Отклонять неизвестный `ServerOptions.ClientAuth` до создания server `tls.Config`, сохранив поведение четырёх допустимых значений.

**Architecture:** Строгая allowlist-проверка добавляется непосредственно в существующий trust boundary `validateServerOptions`; отдельный validator или новая конфигурационная модель не нужны. Нормализованный mode повторно используется существующей проверкой `ClientCA`, а `ServerConfig` и mapping в `crypto/tls` остаются без изменений.

**Tech Stack:** Go 1.26, стандартные `crypto/tls`, `strings` и `testing`; существующий helper `selfSignedPEM`.

**Spec:** `docs/superpowers/specs/2026-08-26-mtls-client-auth-validation-design.md`

**Issue:** https://github.com/st-1dev/zengo/issues/4

## Global Constraints

- Разрешить только `""`, `none`, `verify_if_given` и `require_and_verify`; пустое значение эквивалентно `none`.
- Любой другой непустой mode должен дать понятную ошибку, содержащую его значение.
- Не менять публичные типы, константы, proto schema, TLS minimum version или certificate loading.
- Сохранить обязательный `ClientCA` для двух verifying modes.
- Не добавлять aliases, автоматическое исправление опечаток, новые modes или зависимости.

---

### Task 1: Добавить fail-closed allowlist в server TLS validation

**Files:**
- Modify: `sdk/tlsconfig/tlsconfig_test.go:3-116`
- Modify: `sdk/tlsconfig/tlsconfig.go:254-280`

**Interfaces:**
- Consumes: `ServerOptions.ClientAuth`, `effectiveClientAuth(ClientAuthMode) ClientAuthMode`, существующие `ClientAuth*` constants.
- Produces: `validateServerOptions(*ServerOptions) error`, который отклоняет неизвестный mode до возврата из `ServerConfig`.

- [ ] **Step 1: Зафиксировать allowed и rejected modes тестами**

Добавить `"strings"` в import block `sdk/tlsconfig/tlsconfig_test.go`, заменить `TestServerConfigSupportsMTLS` и добавить regression test:

```go
func TestServerConfigSupportsClientAuthModes(t *testing.T) {
	testCertPEM, testKeyPEM := selfSignedPEM(t)
	tests := []struct {
		name string
		mode ClientAuthMode
		want tls.ClientAuthType
	}{
		{name: "default", want: tls.NoClientCert},
		{name: "none", mode: ClientAuthNone, want: tls.NoClientCert},
		{
			name: "verify if given",
			mode: ClientAuthVerifyIfGiven,
			want: tls.VerifyClientCertIfGiven,
		},
		{
			name: "require and verify",
			mode: ClientAuthRequireAndVerify,
			want: tls.RequireAndVerifyClientCert,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ServerConfig(&ServerOptions{
				Cert:       &Material{InlinePEM: testCertPEM},
				Key:        &Material{InlinePEM: testKeyPEM},
				ClientCA:   &Material{InlinePEM: testCertPEM},
				ClientAuth: tc.mode,
			})
			if err != nil {
				t.Fatal(err)
			}
			if cfg.ClientAuth != tc.want {
				t.Fatalf("ClientAuth = %v, want %v", cfg.ClientAuth, tc.want)
			}
			if tc.want != tls.NoClientCert && cfg.ClientCAs == nil {
				t.Fatal("expected client CAs")
			}
		})
	}
}

func TestServerConfigRejectsUnknownClientAuth(t *testing.T) {
	testCertPEM, testKeyPEM := selfSignedPEM(t)
	mode := ClientAuthMode("require-and-verify")
	_, err := ServerConfig(&ServerOptions{
		Cert:       &Material{InlinePEM: testCertPEM},
		Key:        &Material{InlinePEM: testKeyPEM},
		ClientCA:   &Material{InlinePEM: testCertPEM},
		ClientAuth: mode,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), string(mode)) {
		t.Fatalf("error %q does not contain mode %q", err, mode)
	}
}
```

- [ ] **Step 2: Запустить тесты и подтвердить RED**

Run:

```bash
go test ./sdk/tlsconfig -run 'TestServerConfig(SupportsClientAuthModes|RejectsUnknownClientAuth)$' -count=1
```

Expected: FAIL в `TestServerConfigRejectsUnknownClientAuth` с `expected error`; текущий код успешно возвращает config с `tls.NoClientCert`.

- [ ] **Step 3: Добавить минимальную allowlist-проверку**

Заменить `validateServerOptions` в `sdk/tlsconfig/tlsconfig.go`:

```go
func validateServerOptions(opts *ServerOptions) error {
	if opts == nil {
		return nil
	}
	mode := effectiveClientAuth(opts.ClientAuth)
	switch mode {
	case ClientAuthNone, ClientAuthVerifyIfGiven, ClientAuthRequireAndVerify:
	default:
		return fmt.Errorf("unsupported server tls client auth mode %q", mode)
	}
	err := validateMaterial("cert", opts.Cert)
	if err != nil {
		return err
	}
	err = validateMaterial("key", opts.Key)
	if err != nil {
		return err
	}
	err = validateMaterial("ca", opts.CA)
	if err != nil {
		return err
	}
	err = validateMaterial("client_ca", opts.ClientCA)
	if err != nil {
		return err
	}
	if opts.Cert == nil || opts.Key == nil {
		return fmt.Errorf("server tls cert and key are required")
	}
	if mode != ClientAuthNone && opts.ClientCA == nil {
		return fmt.Errorf("server tls client_ca is required when client auth is enabled")
	}
	return nil
}
```

- [ ] **Step 4: Отформатировать изменённые Go-файлы**

Run:

```bash
gofmt -w sdk/tlsconfig/tlsconfig.go sdk/tlsconfig/tlsconfig_test.go
```

- [ ] **Step 5: Запустить targeted tests и подтвердить GREEN**

Run:

```bash
go test ./sdk/tlsconfig -run 'TestServerConfig(SupportsClientAuthModes|RejectsUnknownClientAuth|RejectsMissingClientCA)$' -count=1
```

Expected: PASS для всех allowed modes, unknown mode и существующей проверки `ClientCA`.

- [ ] **Step 6: Проверить package и весь repository**

Run:

```bash
go test ./sdk/tlsconfig -count=1
go test ./...
```

Expected: обе команды PASS.

- [ ] **Step 7: Зафиксировать implementation commit**

```bash
git add sdk/tlsconfig/tlsconfig.go sdk/tlsconfig/tlsconfig_test.go
git commit -m "fix(tlsconfig): reject unknown client-auth modes"
```
