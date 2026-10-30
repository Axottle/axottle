"use client";
import { Checkbox } from "@/components/new/controls";
import { FormField } from "@/components/new/input";
import { useText } from "@/lib/state";
import type { PermissionGroup, Project } from "@/lib/api";
export function PermissionPicker({
  value,
  onChange,
  groups,
  disabled = false,
}: {
  value: string[];
  onChange: (v: string[]) => void;
  groups: PermissionGroup[];
  disabled?: boolean;
}) {
  const text = useText(),
    all = value.includes("*");
  const labels: Record<string, string> = {
    "accounts.manage": text(
      "Управлять пользователями и доступом",
      "Manage users and access",
    ),
    "settings.view": text("Просматривать настройки", "View settings"),
    "settings.manage": text(
      "Изменять настройки и проекты",
      "Manage settings and projects",
    ),
    "audit.view": text("Просматривать журнал действий", "View audit journal"),
  };
  const known = groups.flatMap((g) => g.permissions),
    other = value.filter((v) => v !== "*" && !known.includes(v));
  return (
    <fieldset className="space-y-3">
      <legend className="mb-3 text-sm font-medium">
        {text("Права доступа", "Permissions")}
      </legend>
      <Checkbox
        checked={all}
        onChange={(v) => onChange(v ? ["*"] : ["settings.view"])}
        label={text("Полный доступ", "Full access")}
        hint={text(
          "Действует в назначенных проектах",
          "Applies within assigned projects",
        )}
        disabled={disabled}
      />
      {!all && (
        <div className="grid gap-3 pt-2 sm:grid-cols-2">
          {known.map((p) => (
            <Checkbox
              key={p}
              label={labels[p] || p}
              checked={value.includes(p)}
              disabled={disabled}
              onChange={(checked) =>
                onChange(checked ? [...value, p] : value.filter((v) => v !== p))
              }
            />
          ))}
        </div>
      )}
      {!all && other.length > 0 && (
        <p className="text-xs text-[var(--axo-text-muted)]">
          {text(
            "Дополнительные права будут сохранены",
            "Additional permissions will be preserved",
          )}
          : {other.join(", ")}
        </p>
      )}
    </fieldset>
  );
}
export function ProjectPicker({
  value,
  onChange,
  projects,
  disabled = false,
}: {
  value: string[];
  onChange: (v: string[]) => void;
  projects: Project[];
  disabled?: boolean;
}) {
  const text = useText();
  return (
    <fieldset className="space-y-3">
      <legend className="mb-3 text-sm font-medium">
        {text("Проекты", "Projects")}
      </legend>
      {projects.map((p) => (
        <Checkbox
          key={p.id}
          checked={value.includes(p.id)}
          disabled={disabled}
          label={p.name}
          onChange={(checked) =>
            onChange(
              checked ? [...value, p.id] : value.filter((v) => v !== p.id),
            )
          }
        />
      ))}
      {value
        .filter((id) => !projects.some((p) => p.id === id))
        .map((id) => (
          <Checkbox key={id} checked label={id} disabled />
        ))}
    </fieldset>
  );
}
