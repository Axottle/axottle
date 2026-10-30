"use client";
import { useState } from "react";
import { RefreshCw } from "lucide-react";
import { Button } from "@/components/new/button";
import { Card } from "@/components/new/card";
import { PageHeader } from "@/components/new/feedback";
import { Input } from "@/components/new/input";
import { ErrorState, InlineLoader } from "@/components/new/state";
import { useI18n } from "@/lib/i18n";
import { usePresentation } from "@/lib/presentation";
import { useResource, useSession, useText } from "@/lib/state";
interface Entry {
  id: number;
  actor_type: string;
  actor_id: string;
  action: string;
  resource_type: string;
  resource_id: string;
  created_at: string;
}
export default function Audit() {
  const presentation = usePresentation();
  const text = useText(),
    { lang } = useI18n(),
    { can } = useSession(),
    [query, setQuery] = useState("");
  const resource = useResource<{ entries: Entry[] }>(
    can("audit.view") ? "audit-log" : null,
  );
  const entries = (resource.data?.entries || []).filter((e) =>
    [e.actor_id, e.action, e.resource_type, e.resource_id]
      .join(" ")
      .toLowerCase()
      .includes(query.toLowerCase()),
  );
  const actionName = (v: string) =>
    (
      ({
        "project.created": text("Создан проект", "Project created"),
        "project.updated": text("Изменён проект", "Project updated"),
        "auth.account.created": text("Создан пользователь", "User created"),
        "auth.account.updated": text("Изменён пользователь", "User updated"),
        "auth.account.deleted": text("Удалён пользователь", "User deleted"),
        "settings.updated": text("Изменены настройки", "Settings changed"),
        "auth.password.changed": text("Изменён пароль", "Password changed"),
      }) as Record<string, string>
    )[v] || v;
  return (
    <>
      <PageHeader
        title={text("Журнал действий", "Audit journal")}
        description={text(
          "Последние 100 действий в доступных проектах",
          "The latest 100 actions in accessible projects",
        )}
        actions={
          <Button
            variant="outline"
            leftIcon={<RefreshCw size={16} />}
            onClick={resource.refresh}
            disabled={resource.loading}
          >
            {text("Обновить", "Refresh")}
          </Button>
        }
      />
      {!can("audit.view") ? (
        <Card>{text("Нет доступа к журналу", "No access to the journal")}</Card>
      ) : (
        <>
          <div className="mb-4 max-w-md">
            <Input
              aria-label={text("Поиск в журнале", "Search journal")}
              placeholder={text(
                "Найти действие или пользователя…",
                "Find an action or user…",
              )}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
          </div>
          {resource.loading ? (
            <InlineLoader />
          ) : resource.error ? (
            <ErrorState message={resource.error} onRetry={resource.refresh} />
          ) : (
            <Card padding="none" className="overflow-x-auto">
              <table className="core-table">
                <thead>
                  <tr>
                    <th>{text("Время", "Time")}</th>
                    <th>{text("Пользователь", "User")}</th>
                    <th>{text("Действие", "Action")}</th>
                    <th>{text("Объект", "Resource")}</th>
                  </tr>
                </thead>
                <tbody>
                  {entries.map((e) => (
                    <tr key={e.id}>
                      <td className="whitespace-nowrap">
                        {new Date(e.created_at).toLocaleString(
                          lang === "ru" ? "ru-RU" : "en-US",
                          { timeZone: presentation.timezone },
                        )}
                      </td>
                      <td>{e.actor_id}</td>
                      <td>{actionName(e.action)}</td>
                      <td className="max-w-64 break-words">
                        {e.resource_type} {e.resource_id}
                      </td>
                    </tr>
                  ))}
                  {!entries.length && (
                    <tr>
                      <td colSpan={4}>{text("Нет действий", "No actions")}</td>
                    </tr>
                  )}
                </tbody>
              </table>
            </Card>
          )}
        </>
      )}
    </>
  );
}
