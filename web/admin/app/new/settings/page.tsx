"use client";
import { useEffect, useState } from "react";
import { useAxoTheme } from "../theme";
import { Card, CardHeader } from "@/components/new/card";
import { Button } from "@/components/new/button";
import { Segmented } from "@/components/new/controls";
import { FormField, Input } from "@/components/new/input";
import { Select } from "@/components/new/select";
import { PageHeader } from "@/components/new/feedback";
import { ErrorState, InlineLoader } from "@/components/new/state";
import { useToast } from "@/components/new/toast";
import { useI18n } from "@/lib/i18n";
import { request } from "@/lib/api";
import { usePresentation } from "@/lib/presentation";
import { useResource, useSession, useText } from "@/lib/state";
export default function Settings() {
  const presentation = usePresentation();
  const text = useText(),
    session = useSession(),
    toast = useToast(),
    theme = useAxoTheme(),
    { lang, setLang } = useI18n();
  const resource = useResource<{ settings: Record<string, string> }>(
    session.can("settings.view") ? "settings/general" : null,
  );
  const [name, setName] = useState(""),
    [language, setLanguage] = useState("ru"),
    [timezone, setTimezone] = useState("UTC"),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  useEffect(() => {
    if (!resource.data) return;
    setName(resource.data.settings["panel.name"] || "Axottle");
    setLanguage(resource.data.settings["panel.language"] || "ru");
    setTimezone(resource.data.settings["panel.timezone"] || "UTC");
  }, [resource.data]);
  async function save(e: React.FormEvent) {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    setError("");
    try {
      new Intl.DateTimeFormat("en", { timeZone: timezone });
      await request("settings/general", {
        method: "PATCH",
        body: {
          settings: {
            "panel.name": name,
            "panel.language": language,
            "panel.timezone": timezone,
          },
        },
      });
      await presentation.refresh();
      toast.success(text("Настройки сохранены", "Settings saved"));
      resource.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }
  const zones = [
    ...new Set([
      timezone,
      "UTC",
      "Europe/Moscow",
      "Asia/Yekaterinburg",
      "Asia/Novosibirsk",
      "Europe/Berlin",
      "America/New_York",
    ]),
  ];
  return (
    <>
      <PageHeader
        title={text("Настройки", "Settings")}
        description={text(
          "Параметры панели и оформление",
          "Panel preferences and appearance",
        )}
      />
      <div className="grid gap-5 lg:grid-cols-2">
        <Card>
          <CardHeader title={text("Общие настройки", "General preferences")} />
          {!session.can("settings.view") ? (
            <p>
              {text(
                "Нет доступа к общим настройкам",
                "No access to general preferences",
              )}
            </p>
          ) : resource.loading ? (
            <InlineLoader />
          ) : resource.error ? (
            <ErrorState message={resource.error} onRetry={resource.refresh} />
          ) : (
            <form onSubmit={save} className="space-y-4">
              <FormField
                label={text("Название панели", "Panel name")}
                htmlFor="panel-name"
              >
                <Input
                  id="panel-name"
                  maxLength={120}
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  disabled={busy || !session.can("settings.manage")}
                />
              </FormField>
              <FormField
                label={text(
                  "Язык панели по умолчанию",
                  "Default panel language",
                )}
              >
                <Select
                  value={language}
                  onChange={setLanguage}
                  disabled={busy || !session.can("settings.manage")}
                  ariaLabel={text(
                    "Язык панели по умолчанию",
                    "Default panel language",
                  )}
                  options={[
                    { value: "ru", label: "Русский" },
                    { value: "en", label: "English" },
                    ...(!["ru", "en"].includes(language)
                      ? [{ value: language, label: language }]
                      : []),
                  ]}
                />
              </FormField>
              <FormField label={text("Часовой пояс", "Timezone")}>
                <Select
                  value={timezone}
                  onChange={setTimezone}
                  disabled={busy || !session.can("settings.manage")}
                  searchable
                  ariaLabel={text("Часовой пояс", "Timezone")}
                  options={zones.map((value) => ({ value, label: value }))}
                />
              </FormField>
              {error && (
                <p
                  role="alert"
                  className="text-sm text-[var(--axo-status-crit)]"
                >
                  {error}
                </p>
              )}
              {session.can("settings.manage") && (
                <Button type="submit" loading={busy}>
                  {text("Сохранить", "Save")}
                </Button>
              )}
            </form>
          )}
        </Card>
        <Card>
          <CardHeader
            title={text("Оформление", "Appearance")}
            description={text(
              "Сохраняется в этом браузере",
              "Saved in this browser",
            )}
          />
          <div className="space-y-6">
            <FormField label={text("Тема", "Theme")}>
              <Segmented
                value={theme.mode}
                onChange={theme.setMode}
                options={[
                  { value: "light", label: text("Светлая", "Light") },
                  { value: "dark", label: text("Тёмная", "Dark") },
                ]}
              />
            </FormField>
            <FormField label={text("Цвет акцента", "Accent color")}>
              <Segmented
                value={theme.palette}
                onChange={theme.setPalette}
                options={[
                  { value: "coral", label: text("Коралловый", "Coral") },
                  { value: "mint", label: text("Мятный", "Mint") },
                ]}
              />
            </FormField>
            <FormField label={text("Язык интерфейса", "Interface language")}>
              <Select
                value={lang}
                onChange={(v) => setLang(v as "ru" | "en")}
                ariaLabel={text("Язык интерфейса", "Interface language")}
                options={[
                  { value: "ru", label: "Русский" },
                  { value: "en", label: "English" },
                ]}
              />
            </FormField>
          </div>
        </Card>
      </div>
    </>
  );
}
