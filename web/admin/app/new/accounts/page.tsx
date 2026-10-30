"use client";
import { useRef, useState } from "react";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/new/button";
import { Card } from "@/components/new/card";
import { Dialog } from "@/components/new/dialog";
import { PageHeader } from "@/components/new/feedback";
import { FormField, Input } from "@/components/new/input";
import { Select } from "@/components/new/select";
import { ErrorState, InlineLoader } from "@/components/new/state";
import {
  WizardDialog,
  WizardSummary,
  type WizardStep,
} from "@/components/new/wizard-dialog";
import { useToast } from "@/components/new/toast";
import { PermissionPicker, ProjectPicker } from "@/components/access-fields";
import { request, type Account, type PermissionGroup } from "@/lib/api";
import { useResource, useSession, useText } from "@/lib/state";
type Step = "identity" | "access" | "review";
export default function Accounts() {
  const text = useText(),
    session = useSession(),
    toast = useToast(),
    writing = useRef(false);
  const resource = useResource<{ accounts: Account[] }>(
      session.can("accounts.manage") ? "auth/accounts" : null,
    ),
    registry = useResource<{ groups: PermissionGroup[] }>("auth/permissions");
  const [open, setOpen] = useState(false),
    [edit, setEdit] = useState<Account | null>(null),
    [remove, setRemove] = useState<Account | null>(null),
    [step, setStep] = useState<Step>("identity"),
    [username, setUsername] = useState(""),
    [display, setDisplay] = useState(""),
    [password, setPassword] = useState(""),
    [permissions, setPermissions] = useState<string[]>(["settings.view"]),
    [projects, setProjects] = useState<string[]>([]),
    [status, setStatus] = useState("active"),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const protectedAccess = Boolean(
    edit && (edit.id === session.me?.id || edit.is_bootstrap_superuser),
  );
  const grantProjects = session.me?.is_bootstrap_superuser
    ? session.projects
    : session.projects.filter((p) => p.id === session.project?.id);
  const steps: WizardStep<Step>[] = [
    {
      id: "identity",
      label: text("Пользователь", "User"),
      heading: text("Кого добавляем в команду?", "Who is joining the team?"),
      hint: text(
        "Укажите имя и данные для входа.",
        "Enter their name and sign-in credentials.",
      ),
    },
    {
      id: "access",
      label: text("Доступ", "Access"),
      heading: text("К чему нужен доступ?", "What access do they need?"),
      hint: text(
        "Выберите проекты и права. Изменения вступят в силу сразу.",
        "Choose projects and permissions. Changes take effect immediately.",
      ),
    },
    {
      id: "review",
      label: text("Проверка", "Review"),
      heading: text("Всё верно?", "Is everything correct?"),
      hint: text(
        "Проверьте аккаунт и доступ перед созданием.",
        "Review the account and access before creation.",
      ),
    },
  ];
  function begin(a?: Account) {
    setUsername(a?.username || "");
    setDisplay(a?.display_name || "");
    setPassword("");
    setPermissions(a?.permissions || ["settings.view"]);
    setProjects(
      a?.project_ids || (session.project ? [session.project.id] : []),
    );
    setStatus(a?.status || "active");
    setStep("identity");
    setError("");
    if (a) setEdit(a);
    else setOpen(true);
  }
  function validateIdentity() {
    const bytes = new TextEncoder().encode(password).length;
    if (!username.trim() || bytes < 8 || bytes > 72) {
      setError(
        text(
          "Укажите логин и пароль от 8 до 72 байт",
          "Enter a username and a password of 8 to 72 bytes",
        ),
      );
      return false;
    }
    return true;
  }
  async function save() {
    if (writing.current) return;
    if (!edit && !validateIdentity()) return;
    if (!protectedAccess && (!permissions.length || !projects.length)) {
      setError(
        text(
          "Выберите хотя бы один проект и одно право",
          "Choose at least one project and permission",
        ),
      );
      return;
    }
    writing.current = true;
    setBusy(true);
    setError("");
    try {
      await request(edit ? `auth/accounts/${edit.id}` : "auth/accounts", {
        method: edit ? "PATCH" : "POST",
        body: edit
          ? {
              display_name: display,
              ...(!protectedAccess
                ? { permissions, project_ids: projects, status }
                : {}),
            }
          : {
              username: username.trim(),
              display_name: display,
              password,
              permissions,
              project_ids: projects,
            },
      });
      setOpen(false);
      setEdit(null);
      setPassword("");
      resource.refresh();
      toast.success(text("Пользователь сохранён", "User saved"));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      writing.current = false;
      setBusy(false);
    }
  }
  function next() {
    setError("");
    if (step === "identity") {
      if (validateIdentity()) setStep("access");
    } else if (step === "access") {
      if (!projects.length || !permissions.length) {
        setError(
          text("Выберите проект и права", "Choose projects and permissions"),
        );
        return;
      }
      setStep("review");
    } else void save();
  }
  async function destroy() {
    if (!remove || writing.current) return;
    writing.current = true;
    setBusy(true);
    setError("");
    try {
      await request(`auth/accounts/${remove.id}`, { method: "DELETE" });
      setRemove(null);
      resource.refresh();
      toast.success(text("Пользователь удалён", "User deleted"));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      writing.current = false;
      setBusy(false);
    }
  }
  const access = (
    <div className="space-y-6">
      <ProjectPicker
        projects={grantProjects}
        value={projects}
        onChange={setProjects}
        disabled={busy || protectedAccess}
      />
      <PermissionPicker
        value={permissions}
        onChange={setPermissions}
        groups={registry.data?.groups || []}
        disabled={busy || protectedAccess}
      />
      {registry.error && (
        <ErrorState message={registry.error} onRetry={registry.refresh} />
      )}
    </div>
  );
  const errorView = error && (
    <p role="alert" className="mt-4 text-sm text-[var(--axo-status-crit)]">
      {error}
    </p>
  );
  return (
    <>
      <PageHeader
        title={text("Пользователи", "Users")}
        description={text(
          "Аккаунты команды, проекты и права доступа",
          "Team accounts, projects and permissions",
        )}
        actions={
          session.can("accounts.manage") && (
            <Button
              leftIcon={<Plus size={17} />}
              disabled={!session.projects.length || registry.loading}
              onClick={() => begin()}
            >
              {text("Добавить пользователя", "Add user")}
            </Button>
          )
        }
      />
      {!session.can("accounts.manage") ? (
        <Card>
          {text(
            "Нет доступа к управлению пользователями",
            "No access to user management",
          )}
        </Card>
      ) : resource.loading ? (
        <InlineLoader />
      ) : resource.error ? (
        <ErrorState message={resource.error} onRetry={resource.refresh} />
      ) : (
        <Card padding="none" className="overflow-x-auto">
          <table className="core-table">
            <thead>
              <tr>
                <th>{text("Пользователь", "User")}</th>
                <th>{text("Статус", "Status")}</th>
                <th>{text("Проекты", "Projects")}</th>
                <th>
                  <span className="sr-only">{text("Действия", "Actions")}</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {resource.data?.accounts.map((a) => (
                <tr key={a.id}>
                  <td>
                    <div className="font-medium">
                      {a.display_name || a.username}
                    </div>
                    <div className="mt-1 text-xs text-[var(--axo-text-muted)]">
                      {a.username}
                      {a.is_bootstrap_superuser &&
                        ` · ${text("Владелец", "Owner")}`}
                    </div>
                  </td>
                  <td>
                    {a.status === "active"
                      ? text("Активен", "Active")
                      : a.status === "disabled"
                        ? text("Отключён", "Disabled")
                        : text("Приглашён", "Invited")}
                  </td>
                  <td>
                    {a.is_bootstrap_superuser
                      ? text("Все проекты", "All projects")
                      : (a.project_ids || [])
                          .map(
                            (id) =>
                              session.projects.find((p) => p.id === id)?.name ||
                              id,
                          )
                          .join(", ")}
                  </td>
                  <td>
                    <div className="flex justify-end gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label={text(
                          "Редактировать пользователя",
                          "Edit user",
                        )}
                        onClick={() => begin(a)}
                      >
                        <Pencil size={16} />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label={text("Удалить пользователя", "Delete user")}
                        disabled={
                          a.is_bootstrap_superuser || a.id === session.me?.id
                        }
                        onClick={() => {
                          setError("");
                          setRemove(a);
                        }}
                      >
                        <Trash2 size={16} />
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
              {!resource.data?.accounts.length && (
                <tr>
                  <td colSpan={4}>
                    {text(
                      "Нет пользователей в выбранном проекте",
                      "No users in the selected project",
                    )}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </Card>
      )}
      {open && (
        <WizardDialog
          title={text("Новый пользователь", "New user")}
          steps={steps}
          step={step}
          onStep={setStep}
          onClose={() => {
            if (!writing.current) {
              setOpen(false);
              setPassword("");
            }
          }}
          busy={busy}
          footer={
            <>
              <Button
                variant="ghost"
                disabled={busy || step === "identity"}
                onClick={() =>
                  setStep(step === "review" ? "access" : "identity")
                }
              >
                {text("Назад", "Back")}
              </Button>
              <Button
                loading={busy}
                disabled={registry.loading || Boolean(registry.error)}
                onClick={next}
              >
                {step === "review"
                  ? text("Создать пользователя", "Create user")
                  : text("Продолжить", "Continue")}
              </Button>
            </>
          }
        >
          <form
            onSubmit={(e) => {
              e.preventDefault();
              next();
            }}
          >
            {step === "identity" && (
              <div className="space-y-4">
                <FormField label={text("Имя", "Name")} htmlFor="new-user-name">
                  <Input
                    id="new-user-name"
                    value={display}
                    onChange={(e) => setDisplay(e.target.value)}
                    disabled={busy}
                  />
                </FormField>
                <FormField
                  label={text("Логин", "Username")}
                  htmlFor="new-user-login"
                >
                  <Input
                    id="new-user-login"
                    autoComplete="off"
                    value={username}
                    onChange={(e) => setUsername(e.target.value)}
                    disabled={busy}
                  />
                </FormField>
                <FormField
                  label={text("Пароль", "Password")}
                  htmlFor="new-user-password"
                >
                  <Input
                    id="new-user-password"
                    type="password"
                    autoComplete="new-password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    disabled={busy}
                  />
                </FormField>
              </div>
            )}
            {step === "access" && access}
            {step === "review" && (
              <WizardSummary
                rows={[
                  {
                    id: "user",
                    label: text("Пользователь", "User"),
                    value: display || username,
                  },
                  {
                    id: "projects",
                    label: text("Проекты", "Projects"),
                    value: projects
                      .map(
                        (id) =>
                          session.projects.find((p) => p.id === id)?.name || id,
                      )
                      .join(", "),
                  },
                  {
                    id: "permissions",
                    label: text("Права", "Permissions"),
                    value: permissions.includes("*")
                      ? text(
                          "Полный доступ в назначенных проектах",
                          "Full access in assigned projects",
                        )
                      : permissions.join(", "),
                  },
                ]}
              />
            )}
            <button
              type="submit"
              className="sr-only"
              tabIndex={-1}
              aria-hidden
              disabled={busy}
            />
          </form>
          {errorView}
        </WizardDialog>
      )}
      <Dialog
        open={Boolean(edit)}
        title={text("Настройки пользователя", "User settings")}
        onClose={() => {
          if (!writing.current) setEdit(null);
        }}
        size="lg"
        footer={
          <Button loading={busy} onClick={save}>
            {text("Сохранить", "Save")}
          </Button>
        }
      >
        <div className="space-y-5">
          <FormField label={text("Имя", "Name")} htmlFor="edit-user-name">
            <Input
              id="edit-user-name"
              value={display}
              onChange={(e) => setDisplay(e.target.value)}
              disabled={busy}
            />
          </FormField>
          <FormField label={text("Статус", "Status")}>
            <Select
              value={status}
              onChange={setStatus}
              ariaLabel={text("Статус пользователя", "User status")}
              disabled={busy || protectedAccess}
              options={[
                { value: "active", label: text("Активен", "Active") },
                { value: "invited", label: text("Приглашён", "Invited") },
                { value: "disabled", label: text("Отключён", "Disabled") },
              ]}
            />
          </FormField>
          {access}
          {errorView}
        </div>
      </Dialog>
      <Dialog
        open={Boolean(remove)}
        title={text("Удалить пользователя?", "Delete this user?")}
        onClose={() => {
          if (!writing.current) setRemove(null);
        }}
        footer={
          <>
            <Button
              variant="ghost"
              disabled={busy}
              onClick={() => setRemove(null)}
            >
              {text("Отмена", "Cancel")}
            </Button>
            <Button variant="danger" loading={busy} onClick={destroy}>
              {text("Удалить", "Delete")}
            </Button>
          </>
        }
      >
        <p className="text-sm">{remove?.display_name || remove?.username}</p>
        {errorView}
      </Dialog>
    </>
  );
}
