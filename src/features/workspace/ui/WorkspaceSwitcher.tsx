import { useState } from "react";
import { IconCheck, IconChevronRight, IconPlus } from "@tabler/icons-react";
import { useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { ApiError } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import { Icon } from "@/shared/ui/icon";
import { useCompanies, useEnvironments, useProjects, useSwitchEnvironment } from "../api/workspace";
import type { Project } from "../model/types";
import { AddOrganizationDialog } from "./AddOrganizationDialog";

/**
 * Переключение рабочего пространства.
 *
 * Уровней для клика два, а не три: компания — это подпись, а не кнопка.
 * На реальных данных компания, проект и окружение часто называются
 * одинаково, и три раскрывающихся уровня с одним и тем же словом
 * заставляли кликать дважды ради однозначного выбора.
 *
 * Выбирается всегда пара «проект + окружение» — именно она определяет,
 * какие данные видит пользователь.
 */
export function WorkspaceSwitcher({
  current,
  onSwitched,
}: {
  current: string;
  onSwitched: () => void;
}) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [adding, setAdding] = useState(false);
  const [open, setOpen] = useState(false);

  const companies = useCompanies(open);
  const switching = useSwitchEnvironment();

  const active = useSession();
  const currentProject = active.getProjectId() ?? "";
  const currentEnvironment = active.getEnvironmentId() ?? "";

  const switchTo = (projectId: string, environmentId: string) => {
    // Уже здесь — лишний запрос и сброс кэша ни к чему.
    if (projectId === currentProject && environmentId === currentEnvironment) {
      return onSwitched();
    }

    switching.mutate(
      { projectId, environmentId },
      {
        onSuccess: () => {
          onSwitched();
          void navigate({ to: "/" });
        },
      },
    );
  };

  const error =
    switching.error instanceof ApiError && typeof switching.error.body === "string"
      ? switching.error.body
      : switching.error?.message;

  return (
    <div className="px-1">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex h-9 w-full items-center gap-2.5 rounded-md px-1 text-left transition-colors hover:bg-surface-hover"
      >
        <span className="grid size-7 shrink-0 place-items-center rounded-md bg-surface-active text-xs font-semibold text-fg-muted">
          {(current[0] ?? "U").toUpperCase()}
        </span>
        <span className="min-w-0 flex-1 truncate text-sm text-fg">{current}</span>
        <Icon
          as={IconChevronRight}
          size={14}
          className={`text-fg-subtle transition-transform ${open ? "rotate-90" : ""}`}
        />
      </button>

      {open && (
        <div className="max-h-64 overflow-y-auto">
          {companies.isLoading && <Line>{t("workspace.loading")}</Line>}
          {companies.error && <Line danger>{t("workspace.loadFailed")}</Line>}
          {!companies.isLoading && (companies.data ?? []).length === 0 && (
            <Line>{t("workspace.noOrganizations")}</Line>
          )}

          {(companies.data ?? []).map((company) => (
            <CompanySection
              key={company.id}
              companyId={company.id}
              companyName={company.name}
              currentProject={currentProject}
              currentEnvironment={currentEnvironment}
              busy={switching.isPending}
              onPick={switchTo}
            />
          ))}
        </div>
      )}

      {error && (
        <p role="alert" className="rounded-md bg-danger-subtle px-2 py-1.5 text-xs text-danger">
          {error}
        </p>
      )}

      <button
        type="button"
        onClick={() => setAdding(true)}
        className="mt-1 mb-1 flex h-8 w-full items-center justify-center gap-1.5 rounded-md border border-border text-sm text-fg transition-colors hover:bg-surface-hover"
      >
        <Icon as={IconPlus} size={14} />
        {t("workspace.addOrganization")}
      </button>

      {adding && <AddOrganizationDialog onClose={() => setAdding(false)} />}
    </div>
  );
}

function CompanySection({
  companyId,
  companyName,
  currentProject,
  currentEnvironment,
  busy,
  onPick,
}: {
  companyId: string;
  companyName: string;
  currentProject: string;
  currentEnvironment: string;
  busy: boolean;
  onPick: (projectId: string, environmentId: string) => void;
}) {
  const { t } = useTranslation();
  const projects = useProjects(companyId);
  const list = projects.data ?? [];

  /*
   * Подпись компании прячем, когда она ничего не добавляет: один проект
   * с тем же названием. На реальных данных это обычный случай, и строка
   * «Test123» над строкой «Test123» — чистый шум.
   */
  const redundant = list.length === 1 && list[0]!.name.trim() === companyName.trim();

  return (
    <>
      {!redundant && (
        <p className="px-2 pt-2 pb-0.5 text-2xs font-medium tracking-wide text-fg-subtle uppercase">
          {companyName}
        </p>
      )}

      {projects.isLoading && <Line indent>{t("workspace.loading")}</Line>}

      {list.map((project) => (
        <ProjectRow
          key={project.id}
          project={project}
          currentProject={currentProject}
          currentEnvironment={currentEnvironment}
          busy={busy}
          onPick={onPick}
        />
      ))}
    </>
  );
}

function ProjectRow({
  project,
  currentProject,
  currentEnvironment,
  busy,
  onPick,
}: {
  project: Project;
  currentProject: string;
  currentEnvironment: string;
  busy: boolean;
  onPick: (projectId: string, environmentId: string) => void;
}) {
  const { t } = useTranslation();
  // Текущий проект раскрыт сразу: чаще всего переключаются внутри него.
  const [open, setOpen] = useState(project.id === currentProject);
  const environments = useEnvironments(project.id, open);
  const list = environments.data ?? [];

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex h-8 w-full items-center gap-2 rounded-md px-2 text-left text-sm text-fg transition-colors hover:bg-surface-hover"
      >
        <Icon
          as={IconChevronRight}
          size={14}
          className={`text-fg-subtle transition-transform ${open ? "rotate-90" : ""}`}
        />
        <span className="flex-1 truncate">{project.name}</span>
        {project.id === currentProject && (
          <span className="text-2xs text-fg-subtle">{t("workspace.currentProject")}</span>
        )}
      </button>

      {open &&
        (environments.isLoading ? (
          <Line indent>{t("workspace.loading")}</Line>
        ) : (
          list.map((environment) => {
            const active =
              project.id === currentProject && environment.id === currentEnvironment;

            return (
              <button
                key={environment.id}
                type="button"
                disabled={busy}
                onClick={() => onPick(project.id, environment.id)}
                className={`flex h-8 w-full items-center gap-2 rounded-md py-0 pr-2 pl-8 text-left text-sm transition-colors hover:bg-surface-hover disabled:opacity-50 ${
                  active ? "text-fg" : "text-fg-muted"
                }`}
              >
                <span
                  className="size-2 shrink-0 rounded-full"
                  style={{ background: environment.color || "var(--color-border-strong)" }}
                />
                <span className="flex-1 truncate">{environment.name}</span>
                {active && <Icon as={IconCheck} size={14} className="text-accent-text" />}
              </button>
            );
          })
        ))}
    </>
  );
}

function Line({
  children,
  indent,
  danger,
}: {
  children: React.ReactNode;
  indent?: boolean;
  danger?: boolean;
}) {
  return (
    <p
      className={`px-2 py-1.5 text-xs ${indent ? "pl-8" : ""} ${
        danger ? "text-danger" : "text-fg-subtle"
      }`}
    >
      {children}
    </p>
  );
}
