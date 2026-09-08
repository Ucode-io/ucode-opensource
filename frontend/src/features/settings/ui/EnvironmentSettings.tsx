import { useState } from "react";
import { IconPencil, IconPlus, IconTrash } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { useEnvironments, type Environment } from "@/features/workspace";
import { useSession } from "@/shared/api/use-session";
import type { TranslationKey } from "@/shared/lib/i18n";
import { Button } from "@/shared/ui/button";
import { CHIP_COLORS, CHIP_HEX, CHIP_STYLES, hexToChipColor } from "@/shared/ui/chip";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";
import { Icon } from "@/shared/ui/icon";
import { Field, Input } from "@/shared/ui/input";
import { Modal } from "@/shared/ui/modal";
import {
  useCreateEnvironment,
  useDeleteEnvironment,
  useUpdateEnvironment,
  type EnvironmentDraft,
} from "../api/environments";
import { useProject } from "../api/project";
import { Empty, SectionHeader, Td, Th } from "./parts";

/**
 * Окружения проекта.
 *
 * Окружение — это изолированный экземпляр ДАННЫХ: схема таблиц одна
 * на проект, а строки свои в каждом (см. CONTEXT). Поэтому здесь
 * заводят не «ещё одну настройку», а вторую копию базы, и удаление
 * уносит её содержимое.
 *
 * Список берётся у переключателя в шапке (features/workspace) — тот же
 * запрос, тот же ключ кэша. Второй список окружений разошёлся бы
 * с первым после первой же правки.
 *
 * Текущее окружение не удаляется: человек сидит в нём, и удалить его
 * значит остаться в сессии, которой некуда смотреть. Переключиться
 * можно в шапке — это переход, а не настройка (docs/adr/0001).
 */
export function EnvironmentSettings() {
  const { t } = useTranslation();
  const store = useSession();
  const projectId = store.getProjectId() ?? "";
  const currentId = store.getEnvironmentId() ?? "";

  const { data: environments = [], isLoading } = useEnvironments(projectId);
  const { project } = useProject();
  const remove = useDeleteEnvironment();

  const [editing, setEditing] = useState<Environment | "new" | null>(null);
  const [deleting, setDeleting] = useState<Environment | null>(null);

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <SectionHeader title={t("environments.title")} hint={t("environments.hint")}>
        <Button size="sm" onClick={() => setEditing("new")}>
          <Icon as={IconPlus} size={14} />
          {t("environments.create")}
        </Button>
      </SectionHeader>

      <div className="min-h-0 flex-1 overflow-auto">
        <table className="w-full border-separate border-spacing-0">
          <thead>
            <tr>
              <Th>{t("environments.name")}</Th>
              <Th>{t("environments.description")}</Th>
              <Th className="w-20" />
            </tr>
          </thead>

          <tbody>
            {isLoading && <Empty text={t("common.loading")} colSpan={3} />}
            {!isLoading && !environments.length && (
              <Empty text={t("environments.empty")} colSpan={3} />
            )}

            {environments.map((environment) => (
              <tr key={environment.id} className="group/row hover:bg-surface-hover">
                <Td>
                  <span className="flex items-center gap-2">
                    {/* Цвет из данных читается как намерение и рисуется
                        токеном палитры: HEX, подобранный под светлую
                        тему, в тёмной становится пятном. */}
                    <span
                      className={`size-2.5 rounded-full ${
                        CHIP_STYLES[hexToChipColor(environment.color)].split(" ")[0]
                      }`}
                    />
                    {environment.name}
                    {environment.id === currentId && (
                      <span className="rounded bg-accent-subtle px-1.5 py-0.5 text-2xs text-accent-text">
                        {t("environments.current")}
                      </span>
                    )}
                  </span>
                </Td>

                <Td className="text-fg-muted">{environment.description}</Td>

                <Td className="text-right">
                  <span className="inline-flex gap-1 opacity-0 transition-opacity group-hover/row:opacity-100 focus-within:opacity-100">
                    <button
                      type="button"
                      onClick={() => setEditing(environment)}
                      aria-label={t("action.edit")}
                      title={t("action.edit")}
                      className="grid size-7 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-surface-active hover:text-fg"
                    >
                      <Icon as={IconPencil} size={14} />
                    </button>

                    <button
                      type="button"
                      disabled={environment.id === currentId}
                      onClick={() => setDeleting(environment)}
                      aria-label={t("action.delete")}
                      title={
                        environment.id === currentId
                          ? t("environments.cantDeleteCurrent")
                          : t("action.delete")
                      }
                      className="grid size-7 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-danger-subtle hover:text-danger disabled:pointer-events-none disabled:opacity-30"
                    >
                      <Icon as={IconTrash} size={14} />
                    </button>
                  </span>
                </Td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {editing && (
        <EnvironmentDialog
          environment={editing === "new" ? null : editing}
          companyId={project?.companyId ?? ""}
          onClose={() => setEditing(null)}
        />
      )}

      {deleting && (
        <ConfirmDialog
          title={t("environments.deleteTitle", { name: deleting.name })}
          description={t("environments.deleteDescription")}
          confirmLabel={t("action.delete")}
          busy={remove.isPending}
          onClose={() => setDeleting(null)}
          onConfirm={() => remove.mutate(deleting.id, { onSuccess: () => setDeleting(null) })}
        />
      )}
    </div>
  );
}

/** Имя, цвет и зачем оно заведено. Больше про окружение и не пишется. */
function EnvironmentDialog({
  environment,
  companyId,
  onClose,
}: {
  environment: Environment | null;
  companyId: string;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const create = useCreateEnvironment(companyId);
  const update = useUpdateEnvironment();

  const [draft, setDraft] = useState<EnvironmentDraft>({
    name: environment?.name ?? "",
    color: environment?.color || CHIP_HEX.blue,
    description: environment?.description ?? "",
  });

  const busy = create.isPending || update.isPending;
  const chosen = hexToChipColor(draft.color);

  return (
    <Modal onClose={onClose}>
      <form
        className="flex w-full max-w-md flex-col gap-4 rounded-xl border border-border bg-surface p-5 shadow-modal"
        onSubmit={(event) => {
          event.preventDefault();
          if (environment) update.mutate({ id: environment.id, draft }, { onSuccess: onClose });
          else create.mutate(draft, { onSuccess: onClose });
        }}
      >
        <h2 className="text-base font-semibold">
          {environment ? t("environments.editTitle") : t("environments.create")}
        </h2>

        <Field label={t("environments.name")} hint={t("environments.nameHint")}>
          <Input
            autoFocus
            required
            value={draft.name}
            onChange={(event) => setDraft({ ...draft, name: event.target.value })}
          />
        </Field>

        {/*
          Цвет выбирается из палитры, а не пипеткой: в данных он лежит
          HEX'ом и его читает не только эта админка, но подобранный
          вручную оттенок ломается в тёмной теме — как у вариантов поля.
        */}
        <Field label={t("environments.color")}>
          <div className="flex flex-wrap gap-1.5">
            {CHIP_COLORS.map((color) => (
              <button
                key={color}
                type="button"
                onClick={() => setDraft({ ...draft, color: CHIP_HEX[color] })}
                aria-label={t(`color.${color}` as TranslationKey)}
                title={t(`color.${color}` as TranslationKey)}
                aria-pressed={chosen === color}
                className={`size-6 rounded-full transition-transform ${
                  CHIP_STYLES[color].split(" ")[0]
                } ${chosen === color ? "ring-2 ring-accent ring-offset-2 ring-offset-surface" : ""}`}
              />
            ))}
          </div>
        </Field>

        <Field label={t("environments.description")}>
          <Input
            value={draft.description}
            onChange={(event) => setDraft({ ...draft, description: event.target.value })}
          />
        </Field>

        <div className="flex justify-end gap-2">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("action.cancel")}
          </Button>
          <Button type="submit" disabled={!draft.name.trim() || busy}>
            {busy ? t("common.saving") : environment ? t("action.save") : t("action.create")}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
