"use client";
import { useRef, useState } from "react";
import { Pencil, Plus } from "lucide-react";
import { Button } from "@/components/new/button";
import { Card } from "@/components/new/card";
import { Dialog } from "@/components/new/dialog";
import { PageHeader } from "@/components/new/feedback";
import { FormField, Input, Textarea } from "@/components/new/input";
import {
  WizardDialog,
  WizardSummary,
  type WizardStep,
} from "@/components/new/wizard-dialog";
import { request, type Project } from "@/lib/api";
import { useSession, useText } from "@/lib/state";
import { useToast } from "@/components/new/toast";
type Step = "name" | "description" | "review";
export default function Projects() {
  const session = useSession(),
    text = useText(),
    toast = useToast(),
    writing = useRef(false);
  const [open, setOpen] = useState(false),
    [edit, setEdit] = useState<Project | null>(null),
    [step, setStep] = useState<Step>("name"),
    [name, setName] = useState(""),
    [description, setDescription] = useState(""),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  const canEdit = session.can("settings.manage");
  const steps: WizardStep<Step>[] = [
    {
      id: "name",
      label: text("Название", "Name"),
      heading: text("Как назовём проект?", "What is the project called?"),
      hint: text(
        "Название поможет команде выбрать нужное рабочее пространство.",
        "A name helps your team choose the right workspace.",
      ),
    },
    {
      id: "description",
      label: text("Описание", "Description"),
      heading: text("Что стоит знать команде?", "What should your team know?"),
      hint: text(
        "Добавьте описание или оставьте поле пустым.",
        "Add a description, or leave it blank.",
      ),
    },
    {
      id: "review",
      label: text("Проверка", "Review"),
      heading: text("Создаём проект?", "Create this project?"),
      hint: text(
        "Проверьте название и описание перед сохранением.",
        "Review the name and description before saving.",
      ),
    },
  ];
  function begin(p?: Project) {
    setName(p?.name || "");
    setDescription(p?.description || "");
    setError("");
    setStep("name");
    if (p) setEdit(p);
    else setOpen(true);
  }
  async function save() {
    if (writing.current) return;
    if (
      !name.trim() ||
      [...name.trim()].length > 120 ||
      [...description.trim()].length > 2000
    ) {
      setError(
        text(
          "Проверьте название и длину описания",
          "Check the project name and description length",
        ),
      );
      return;
    }
    writing.current = true;
    setBusy(true);
    setError("");
    try {
      const p = await request<Project>(
        edit ? `projects/${edit.id}` : "projects",
        {
          method: edit ? "PATCH" : "POST",
          body: { name: name.trim(), description: description.trim() },
        },
      );
      await session.refresh();
      session.choose(p.id);
      setOpen(false);
      setEdit(null);
      toast.success(text("Проект сохранён", "Project saved"));
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      writing.current = false;
      setBusy(false);
    }
  }
  function next() {
    if (step === "name") {
      if (!name.trim()) {
        setError(text("Укажите название", "Enter a name"));
        return;
      }
      setStep("description");
    } else if (step === "description") setStep("review");
    else void save();
  }
  const fields = (
    <div className="space-y-4">
      <FormField label={text("Название", "Name")} htmlFor="edit-project-name">
        <Input
          id="edit-project-name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          disabled={busy}
          maxLength={120}
        />
      </FormField>
      <FormField
        label={text("Описание", "Description")}
        htmlFor="edit-project-description"
      >
        <Textarea
          id="edit-project-description"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          disabled={busy}
          maxLength={2000}
        />
      </FormField>
    </div>
  );
  return (
    <>
      <PageHeader
        title={text("Проекты", "Projects")}
        description={text(
          "Рабочие пространства и доступ команды",
          "Workspaces and team access",
        )}
        actions={
          session.me?.is_bootstrap_superuser && (
            <Button leftIcon={<Plus size={17} />} onClick={() => begin()}>
              {text("Создать проект", "Create project")}
            </Button>
          )
        }
      />
      {!session.projects.length ? (
        <Card>
          <p className="text-sm text-[var(--axo-text-muted)]">
            {text(
              "Пока нет доступных проектов. Владелец может создать первый проект и пригласить команду.",
              "No projects are available yet. The owner can create the first project and add the team.",
            )}
          </p>
        </Card>
      ) : (
        <div className="grid gap-4 md:grid-cols-2">
          {session.projects.map((p) => (
            <Card key={p.id}>
              <div className="flex items-start justify-between gap-3">
                <button
                  onClick={() => session.choose(p.id)}
                  className="axo-focus-ring rounded-lg text-left"
                >
                  <h2 className="text-lg font-semibold">{p.name}</h2>
                  <p className="mt-2 text-sm text-[var(--axo-text-muted)] break-words">
                    {p.description || text("Без описания", "No description")}
                  </p>
                  {session.project?.id === p.id && (
                    <span className="mt-3 inline-block text-xs text-[var(--axo-accent)]">
                      {text("Выбран", "Selected")}
                    </span>
                  )}
                </button>
                {canEdit && (
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label={text("Редактировать проект", "Edit project")}
                    onClick={() => begin(p)}
                  >
                    <Pencil size={17} />
                  </Button>
                )}
              </div>
            </Card>
          ))}
        </div>
      )}
      {open && (
        <WizardDialog
          title={text("Новый проект", "New project")}
          steps={steps}
          step={step}
          onStep={setStep}
          onClose={() => {
            if (!writing.current) setOpen(false);
          }}
          busy={busy}
          footer={
            <>
              <Button
                variant="ghost"
                disabled={busy || step === "name"}
                onClick={() =>
                  setStep(step === "review" ? "description" : "name")
                }
              >
                {text("Назад", "Back")}
              </Button>
              <Button loading={busy} onClick={next}>
                {step === "review"
                  ? text("Создать проект", "Create project")
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
            {step === "name" && (
              <FormField
                label={text("Название", "Name")}
                htmlFor="new-project-name"
              >
                <Input
                  id="new-project-name"
                  autoFocus
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  maxLength={120}
                  disabled={busy}
                />
              </FormField>
            )}
            {step === "description" && (
              <FormField
                label={text("Описание", "Description")}
                htmlFor="new-project-description"
              >
                <Textarea
                  id="new-project-description"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  maxLength={2000}
                  disabled={busy}
                />
              </FormField>
            )}
            {step === "review" && (
              <WizardSummary
                rows={[
                  {
                    id: "name",
                    label: text("Название", "Name"),
                    value: name.trim(),
                  },
                  {
                    id: "description",
                    label: text("Описание", "Description"),
                    value: description.trim() || "—",
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
          {error && (
            <p role="alert" className="text-sm text-[var(--axo-status-crit)]">
              {error}
            </p>
          )}
        </WizardDialog>
      )}
      <Dialog
        open={Boolean(edit)}
        title={text("Настройки проекта", "Project settings")}
        onClose={() => {
          if (!writing.current) setEdit(null);
        }}
        footer={
          <Button loading={busy} onClick={save}>
            {text("Сохранить", "Save")}
          </Button>
        }
      >
        {fields}
        {error && (
          <p
            role="alert"
            className="mt-4 text-sm text-[var(--axo-status-crit)]"
          >
            {error}
          </p>
        )}
      </Dialog>
    </>
  );
}
