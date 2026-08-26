# Non-Destructive Service Init Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Гарантировать, что `InitService` не изменяет непустой target, одновременно сохранив генерацию в пустой и отсутствующий каталоги.

**Architecture:** Корень генерации один раз нормализуется через `filepath.Clean`, затем проверяется `os.ReadDir` до первого filesystem write и тот же путь используется внутри template walk. Защита находится в общем `InitService`, поэтому CLI получает её без дублирования; staging, force mode и новый helper не добавляются.

**Tech Stack:** Go 1.26, стандартные `os`, `path/filepath`, `io/fs`, `testing`; существующая embedded template FS.

**Spec:** `docs/superpowers/specs/2026-08-26-init-nonempty-target-design.md`

**Issue:** https://github.com/st-1dev/zengo/issues/5

## Global Constraints

- Проверять тот же `targetDir`, который используется как корень всех записей, включая `dir == ""` как `.`.
- Возвращать ошибку до первого `MkdirAll` или `WriteFile`, если target существует и содержит любую entry.
- Сохранить поддержку существующего пустого и отсутствующего target.
- Не менять сигнатуру `InitService` и не ослаблять filesystem errors.
- Не добавлять `--force`, overlay, backup, staging, locking, интерактивный prompt или зависимости.

---

### Task 1: Отклонить непустой target до обхода templates

**Files:**
- Modify: `internal/scaffold/scaffold_test.go:3-116`
- Modify: `internal/scaffold/scaffold.go:45-100`

**Interfaces:**
- Consumes: `InitService(name, dir string, opts InitOptions) error`, `os.ReadDir`, `filepath.Clean`.
- Produces: та же публичная сигнатура `InitService`; непустой target возвращает contextual error без filesystem mutations.

- [ ] **Step 1: Написать regression tests для существующего, пустого и отсутствующего target**

Добавить в `internal/scaffold/scaffold_test.go`:

```go
func TestInitServiceRefusesNonEmptyTarget(t *testing.T) {
	dir := t.TempDir()
	sentinelPath := filepath.Join(dir, "go.mod")
	want := "module existing\n"
	err := os.WriteFile(sentinelPath, []byte(want), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	err = InitService("demo-service", dir, InitOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("error = %q", err)
	}
	got, err := os.ReadFile(sentinelPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("go.mod = %q, want %q", got, want)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "go.mod" {
		t.Fatalf("target entries = %v, want only go.mod", entries)
	}
}

func TestInitServiceRefusesNonEmptyCurrentDirectory(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("keep\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	err = InitService("demo-service", "", InitOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "existing.txt" {
		t.Fatalf("target entries = %v, want only existing.txt", entries)
	}
}

func TestInitServiceCreatesMissingTarget(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "service")
	err := InitService("demo-service", dir, InitOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(dir, "zengo.textproto")); err != nil {
		t.Fatalf("expected zengo.textproto: %v", err)
	}
}
```

- [ ] **Step 2: Запустить тесты и подтвердить RED**

Run:

```bash
go test ./internal/scaffold -run 'TestInitService(RefusesNonEmptyTarget|RefusesNonEmptyCurrentDirectory|CreatesMissingTarget)$' -count=1
```

Expected: FAIL в двух `RefusesNonEmpty*` tests, потому что текущий код возвращает `nil` и пишет scaffold в test-каталоги; `CreatesMissingTarget` проходит.

- [ ] **Step 3: Добавить preflight и единый нормализованный root**

В начале `InitService` до вычисления template replacements добавить:

```go
	targetDir := filepath.Clean(dir)
	entries, err := os.ReadDir(targetDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect init target %q: %w", targetDir, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("init target %q is not empty", targetDir)
	}
```

В callback `fs.WalkDir` заменить единственное построение пути:

```go
		target := filepath.Join(targetDir, replacer.Replace(rel))
```

Не менять остальные ветки template walk.

- [ ] **Step 4: Отформатировать изменённые Go-файлы**

Run:

```bash
gofmt -w internal/scaffold/scaffold.go internal/scaffold/scaffold_test.go
```

- [ ] **Step 5: Запустить targeted tests и подтвердить GREEN**

Run:

```bash
go test ./internal/scaffold -run 'TestInitService(RefusesNonEmptyTarget|RefusesNonEmptyCurrentDirectory|CreatesMissingTarget)$' -count=1
```

Expected: PASS; оба непустых target не изменены, отсутствующий target создан.

- [ ] **Step 6: Проверить package и весь repository**

Run:

```bash
go test ./internal/scaffold -count=1
go test ./...
```

Expected: обе команды PASS, включая существующие empty-directory textproto/YAML tests.

- [ ] **Step 7: Зафиксировать implementation commit**

```bash
git add internal/scaffold/scaffold.go internal/scaffold/scaffold_test.go
git commit -m "fix(scaffold): refuse non-empty init targets"
```
