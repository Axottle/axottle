"use client";
import Link from "next/link";
import { usePresentation } from "@/lib/presentation";
import { FolderOpen, ShieldCheck, UsersRound } from "lucide-react";
import { Card, CardHeader } from "@/components/new/card";
import { PageHeader } from "@/components/new/feedback";
import { useSession, useText } from "@/lib/state";
export default function Overview() {
  const presentation = usePresentation();
  const { me, projects, project, can } = useSession(),
    text = useText();
  return (
    <>
      <PageHeader
        title={text("Добро пожаловать", "Welcome")}
        description={presentation.name}
      />
      <div className="grid gap-5 lg:grid-cols-3">
        <Card>
          <CardHeader
            title={text("Проекты", "Projects")}
            action={<FolderOpen size={20} />}
          />
          <p className="mb-3 text-4xl font-semibold">{projects.length}</p>
          <Link
            href="/new/projects"
            className="text-sm text-[var(--axo-accent)]"
          >
            {text("Открыть проекты", "Open projects")}
          </Link>
        </Card>
        <Card>
          <CardHeader title={text("Текущий проект", "Current project")} />
          <p className="font-semibold">
            {project?.name ||
              text("Создайте первый проект", "Create your first project")}
          </p>
          <p className="mt-2 text-sm text-[var(--axo-text-muted)]">
            {project?.description ||
              text(
                "Выбирайте проект в верхней панели.",
                "Choose a project in the top bar.",
              )}
          </p>
        </Card>
        <Card>
          <CardHeader
            title={text("Ваш аккаунт", "Your account")}
            action={<ShieldCheck size={20} />}
          />
          <p className="font-semibold">{me?.display_name || me?.username}</p>
          <Link
            href="/new/profile"
            className="mt-4 inline-block text-sm text-[var(--axo-accent)]"
          >
            {text(
              "Пароль и двухфакторная защита",
              "Password and two-factor protection",
            )}
          </Link>
        </Card>
      </div>
      {can("accounts.manage") && (
        <Card className="mt-5">
          <CardHeader
            title={text("Доступ команды", "Team access")}
            action={<UsersRound size={20} />}
            description={text(
              "Добавляйте пользователей и назначайте им проекты и права.",
              "Add users and assign projects and permissions.",
            )}
          />
          <Link
            href="/new/accounts"
            className="text-sm text-[var(--axo-accent)]"
          >
            {text("Управлять пользователями", "Manage users")}
          </Link>
        </Card>
      )}
    </>
  );
}
