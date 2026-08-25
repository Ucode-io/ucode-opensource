import { useState } from "react";
import { IconEye, IconPencil, IconPlus, IconTrash } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";
import { Icon } from "@/shared/ui/icon";
import {
  useDeleteResource,
  useResources,
  RESOURCE_LABELS,
  RESOURCE_SPECS,
  type Resource,
} from "../../api/resources";
import { Empty, SectionHeader, Td, Th } from "../parts";
import { CreateResourceDialog, EditResourceDialog } from "./ResourceDialog";
import { ResourceIcon } from "./ResourceIcon";
import { ResourceTypePicker } from "./ResourceTypePicker";

/**
 * Ресурсы проекта — чужие службы, которыми он пользуется.
 *
 * Отправка кодов подтверждения, репозиторий с кодом, панель аналитики.
 * Строка списка — это «чем проект пользуется», а не «что у него внутри»:
 * база самого проекта тоже здесь, но только чтобы её было видно, —
 * заводится и удаляется она вместе с окружением.
 *
 * Ресурс принадлежит ОКРУЖЕНИЮ: заведённый в dev в prod не появится.
 * Отдельного переключателя окружения тут нет — он в шапке приложения,
 * и второй, свой, означал бы два разных ответа на вопрос «где я».
 */
export function ResourceSettings() {
  const { t } = useTranslation();
  const { resources, isLoading } = useResources();
  const remove = useDeleteResource();

  /** Выбранный в первом шаге тип. Пусто — форма создания закрыта. */
  const [creating, setCreating] = useState("");
  const [picking, setPicking] = useState(false);
  const [editing, setEditing] = useState<Resource | null>(null);
  const [deleting, setDeleting] = useState<Resource | null>(null);

  const clashing = duplicatedSenders(resources);

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <SectionHeader title={t("resources.title")} hint={t("resources.hint")}>
        <Button size="sm" onClick={() => setPicking(true)}>
          <Icon as={IconPlus} size={14} />
          {t("resources.create")}
        </Button>
      </SectionHeader>

      {clashing.length > 0 && (
        <p className="shrink-0 border-b border-border bg-warning-subtle px-4 py-2 text-xs text-warning">
          {t("resources.duplicate", { kinds: clashing.join(", ") })}
        </p>
      )}

      <div className="min-h-0 flex-1 overflow-auto">
        <table className="w-full border-separate border-spacing-0">
          <thead>
            <tr>
              <Th>{t("settings.name")}</Th>
              <Th className="w-40">{t("resources.type")}</Th>
              <Th>{t("resources.note")}</Th>
              <Th className="w-20" />
            </tr>
          </thead>

          <tbody>
            {isLoading && <Empty text={t("common.loading")} colSpan={4} />}
            {!isLoading && !resources.length && <Empty text={t("resources.empty")} colSpan={4} />}

            {resources.map((resource) => {
              const spec = RESOURCE_SPECS[resource.kind];
              /* Незнакомый тип открывается на чтение: его настроек мы
                 не знаем, а сохранение стёрло бы их целиком. */
              const managed = !spec || Boolean(spec.managed);

              return (
                <tr key={resource.id} className="group/row hover:bg-surface-hover">
                  <Td>
                    <span className="flex items-center gap-2">
                      <span className="text-fg-subtle">
                        <ResourceIcon kind={resource.kind} size={14} />
                      </span>
                      <span className="truncate">{resource.name}</span>
                    </span>
                  </Td>

                  <Td className="text-fg-muted">
                    {RESOURCE_LABELS[resource.kind] ?? resource.kind}
                  </Td>

                  <Td className="text-xs text-fg-subtle">
                    {spec?.system
                      ? t("resources.systemShort")
                      : managed
                        ? t("resources.managedShort")
                        : ""}
                  </Td>

                  <Td className="text-right">
                    <span className="inline-flex gap-1 opacity-0 transition-opacity group-hover/row:opacity-100 focus-within:opacity-100">
                      <button
                        type="button"
                        onClick={() => setEditing(resource)}
                        aria-label={t(managed ? "action.open" : "action.edit")}
                        title={t(managed ? "action.open" : "action.edit")}
                        className="grid size-7 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-surface-active hover:text-fg"
                      >
                        <Icon as={managed ? IconEye : IconPencil} size={14} />
                      </button>

                      {/* Удаления нет там, где оно не сработает: строку
                          проекта эта ручка не найдёт, а Telegram отклонит
                          со ссылкой на свою. */}
                      {!spec?.system && !spec?.noDelete && (
                        <button
                          type="button"
                          onClick={() => setDeleting(resource)}
                          aria-label={t("action.delete")}
                          title={t("action.delete")}
                          className="grid size-7 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-danger-subtle hover:text-danger"
                        >
                          <Icon as={IconTrash} size={14} />
                        </button>
                      )}
                    </span>
                  </Td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {picking && (
        <ResourceTypePicker
          onClose={() => setPicking(false)}
          onPick={(kind) => {
            setPicking(false);
            setCreating(kind);
          }}
        />
      )}

      {creating && <CreateResourceDialog kind={creating} onClose={() => setCreating("")} />}

      {editing && <EditResourceDialog id={editing.id} onClose={() => setEditing(null)} />}

      {deleting && (
        <ConfirmDialog
          title={t("resources.deleteTitle", { name: deleting.name })}
          description={t("resources.deleteDescription")}
          confirmLabel={t("action.delete")}
          busy={remove.isPending}
          onClose={() => setDeleting(null)}
          onConfirm={() => remove.mutate(deleting.id, { onSuccess: () => setDeleting(null) })}
        />
      )}
    </div>
  );
}

/**
 * Отправители кодов, заведённые дважды.
 *
 * Код берётся из ПЕРВОГО ресурса типа (`session_service_v2.go:1104`),
 * а порядка у списка нет вовсе — в запросе нет ORDER BY. То есть при
 * двух ресурсах одного типа неизвестно не только «какой лишний»,
 * но и «какой из них сейчас работает». Молчать об этом нельзя.
 */
function duplicatedSenders(resources: Resource[]): string[] {
  const counts = new Map<string, number>();

  for (const resource of resources) {
    if (RESOURCE_SPECS[resource.kind]?.group !== "otp") continue;
    counts.set(resource.kind, (counts.get(resource.kind) ?? 0) + 1);
  }

  return [...counts]
    .filter(([, count]) => count > 1)
    .map(([kind]) => RESOURCE_LABELS[kind] ?? kind);
}
