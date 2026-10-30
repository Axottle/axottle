"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  FileClock,
  FolderOpen,
  Home,
  LogOut,
  Menu,
  Moon,
  Settings2,
  Sun,
  UserRound,
  UsersRound,
  X,
} from "lucide-react";
import { useAxoTheme } from "@/app/new/theme";
import { Button } from "@/components/new/button";
import { Select } from "@/components/new/select";
import { ErrorState, InlineLoader } from "@/components/new/state";
import { useI18n } from "@/lib/i18n";
import { useScrollLock } from "@/lib/new/use-scroll-lock";
import { request } from "@/lib/api";
import { useSession, useText } from "@/lib/state";
import { clearAuthed } from "@/lib/session";
import { uiAsset } from "@/lib/ui-assets";
export function Shell({ children }: { children: React.ReactNode }) {
  const session = useSession(),
    text = useText(),
    path = usePathname().replace(/\/$/, "");
  const { mode, toggleMode } = useAxoTheme(),
    { lang, setLang } = useI18n();
  const [mobile, setMobile] = useState(false),
    [leaving, setLeaving] = useState(false),
    [exitError, setExitError] = useState("");
  useScrollLock(mobile);
  useEffect(() => {
    setMobile(false);
  }, [path]);
  useEffect(() => {
    const close = (e: KeyboardEvent) => {
      if (e.key === "Escape") setMobile(false);
    };
    window.addEventListener("keydown", close);
    return () => window.removeEventListener("keydown", close);
  }, []);
  const nav = [
    { href: "/new", label: text("Обзор", "Overview"), icon: Home },
    {
      href: "/new/projects",
      label: text("Проекты", "Projects"),
      icon: FolderOpen,
    },
    ...(session.can("accounts.manage")
      ? [
          {
            href: "/new/accounts",
            label: text("Пользователи", "Users"),
            icon: UsersRound,
          },
        ]
      : []),
    ...(session.can("settings.view")
      ? [
          {
            href: "/new/settings",
            label: text("Настройки", "Settings"),
            icon: Settings2,
          },
        ]
      : []),
    ...(session.can("audit.view")
      ? [
          {
            href: "/new/audit-log",
            label: text("Журнал действий", "Audit journal"),
            icon: FileClock,
          },
        ]
      : []),
    {
      href: "/new/profile",
      label: text("Мой аккаунт", "My account"),
      icon: UserRound,
    },
  ];
  async function logout() {
    if (leaving) return;
    setLeaving(true);
    setExitError("");
    try {
      await request("auth/logout", { method: "POST" });
      clearAuthed();
      window.location.replace("/login/");
    } catch (err) {
      setExitError(err instanceof Error ? err.message : String(err));
      setLeaving(false);
    }
  }
  if (!session.ready)
    return (
      <div data-axo-new className="p-10">
        <InlineLoader />
      </div>
    );
  if (session.error)
    return (
      <div data-axo-new className="p-10">
        <ErrorState
          message={session.error}
          onRetry={() => window.location.reload()}
        />
      </div>
    );
  if (!session.me) return null;
  return (
    <div data-axo-new className="core-shell">
      {mobile && (
        <button
          type="button"
          aria-label={text("Закрыть меню", "Close menu")}
          className="fixed inset-0 z-80 bg-black/35 md:hidden"
          onClick={() => setMobile(false)}
        />
      )}
      <aside
        className="core-nav"
        data-open={mobile}
        id="core-navigation"
        aria-label={text("Навигация", "Navigation")}
      >
        <div className="mb-8 flex items-center gap-3 px-3">
          <img
            src={uiAsset("/logo.png")}
            alt=""
            className="h-11 w-11 object-contain"
          />
          <span className="axo-brand-wordmark text-[var(--axo-text)]">
            Axottle
          </span>
          <button
            className="ml-auto md:hidden axo-focus-ring"
            aria-label={text("Закрыть меню", "Close menu")}
            onClick={() => setMobile(false)}
          >
            <X size={20} />
          </button>
        </div>
        <nav className="flex flex-col gap-1">
          {nav.map((item) => (
            <Link
              prefetch={false}
              href={item.href}
              key={item.href}
              aria-current={path === item.href ? "page" : undefined}
            >
              <item.icon size={18} />
              {item.label}
            </Link>
          ))}
        </nav>
        <div className="mt-auto space-y-3 px-3 pt-8">
          <p className="truncate text-sm font-medium">
            {session.me.display_name || session.me.username}
          </p>
          <Button
            variant="ghost"
            onClick={logout}
            loading={leaving}
            leftIcon={<LogOut size={16} />}
          >
            {text("Выйти", "Sign out")}
          </Button>
          {exitError && (
            <p role="alert" className="text-xs text-[var(--axo-status-crit)]">
              {exitError}
            </p>
          )}
        </div>
      </aside>
      <div className="min-w-0">
        <header className="core-topbar">
          <Button
            variant="ghost"
            size="icon"
            className="md:hidden"
            aria-label={text("Открыть меню", "Open menu")}
            aria-expanded={mobile}
            aria-controls="core-navigation"
            onClick={() => setMobile(true)}
          >
            <Menu size={21} />
          </Button>
          <div className="w-64 max-w-[55vw]">
            <Select
              value={session.project?.id || ""}
              onChange={session.choose}
              options={session.projects.map((p) => ({
                value: p.id,
                label: p.name,
              }))}
              ariaLabel={text("Текущий проект", "Current project")}
              placeholder={text("Нет проектов", "No projects")}
              disabled={!session.projects.length}
            />
          </div>
          <div className="ml-auto flex items-center gap-2">
            <Button
              variant="ghost"
              size="sm"
              aria-label={
                lang === "ru" ? "Switch to English" : "Переключить на русский"
              }
              onClick={() => setLang(lang === "ru" ? "en" : "ru")}
            >
              {lang.toUpperCase()}
            </Button>
            <Button
              variant="ghost"
              size="icon"
              aria-label={text("Переключить тему", "Toggle theme")}
              onClick={toggleMode}
            >
              {mode === "dark" ? <Sun size={18} /> : <Moon size={18} />}
            </Button>
          </div>
        </header>
        <main className="core-content">{children}</main>
      </div>
    </div>
  );
}
