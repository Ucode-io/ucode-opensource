import { useDeferredValue, useState } from "react";
import { IconExternalLink, IconHistory, IconPlus, IconTrash } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import {
  isPipelineDone,
  loginSubdomain,
  useBindLoginMicrofront,
  useCreateMicrofrontend,
  useDeleteMicrofrontend,
  useFilesAtCommit,
  useLoginMicrofront,
  useManagedMicrofrontends,
  useMicrofrontendCommits,
  usePipelineStatus,
  usePromoteChanges,
  usePromoteMicrofrontend,
  useRevertMicrofrontend,
  type ManagedMicrofrontend,
} from "@/features/microfrontend";
import { Button } from "@/shared/ui/button";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";
import { Icon } from "@/shared/ui/icon";
import { Field, Input } from "@/shared/ui/input";
import { Modal } from "@/shared/ui/modal";
import { Empty, SectionHeader, Td, Th } from "./parts";

/**
 * Микрофронтенды: список, заведение, версии с откатом, публикация.
 *
 * Чего здесь нет — правки записи и редактора файлов. Обе причины
 * в бэкенде и обе записаны в `microfrontend/api/manage`: `PUT` сломан
 * (на PostgreSQL уходит пустая структура, на MongoDB обнуляет половину
 * колонок), а ручки записи файлов шлюз не проксирует вовсе. Показ
 * файлов при этом есть — снимком версии.
 */
export function MicrofrontendSettings() {
  const { t } = useTranslation();
  const { items, isLoading, error } = useManagedMicrofrontends();
  const remove = useDeleteMicrofrontend();

  const [creating, setCreating] = useState(false);
  const [deleting, setDeleting] = useState<ManagedMicrofrontend | null>(null);
  const [versions, setVersions] = useState<ManagedMicrofrontend | null>(null);

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <SectionHeader title={t("microfrontends.title")} hint={t("microfrontends.hint")}>
        <Button size="sm" onClick={() => setCreating(true)}>
          <Icon as={IconPlus} size={14} />
          {t("microfrontends.create")}
        </Button>
      </SectionHeader>

      <div className="min-h-0 flex-1 overflow-auto">
        <table className="w-full border-separate border-spacing-0">
          <thead>
            <tr>
              <Th>{t("microfrontends.name")}</Th>
              <Th>{t("microfrontends.url")}</Th>
              <Th className="w-28">{t("functions.branch")}</Th>
              <Th className="w-36">{t("functions.pipeline")}</Th>
              <Th className="w-56" />
            </tr>
          </thead>

          <tbody>
            {isLoading && <Empty text={t("common.loading")} colSpan={5} />}
            {!isLoading && !items.length && (
              <Empty text={error ?? t("microfrontends.empty")} colSpan={5} />
            )}

            {items.map((item) => (
              <tr key={item.id} className="hover:bg-surface-hover">
                <Td>{item.name || t("functions.untitled")}</Td>
                <Td className="font-mono text-xs text-fg-muted">
                  {item.url ? (
                    <a
                      href={`https://${item.url}`}
                      target="_blank"
                      rel="noreferrer"
                      className="inline-flex items-center gap-1 hover:underline"
                    >
                      {item.url}
                      <Icon as={IconExternalLink} size={12} />
                    </a>
                  ) : (
                    "—"
                  )}
                </Td>
                <Td className="text-fg-muted">{item.branch}</Td>
                <Td>
                  <span
                    title={item.errorMessage || undefined}
                    className={`text-2xs ${
                      /fail|error|cancel/i.test(item.pipelineStatus)
                        ? "text-danger"
                        : "text-fg-muted"
                    }`}
                  >
                    {item.pipelineStatus || "—"}
                  </span>
                </Td>
                <Td>
                  <div className="flex items-center justify-end gap-1">
                    <PromoteButton item={item} />

                    <button
                      type="button"
                      onClick={() => setVersions(item)}
                      aria-label={t("microfrontends.versions")}
                      title={t("microfrontends.versions")}
                      className="grid size-7 shrink-0 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-surface-active hover:text-fg"
                    >
                      <Icon as={IconHistory} size={14} />
                    </button>

                    <button
                      type="button"
                      onClick={() => setDeleting(item)}
                      aria-label={t("action.delete")}
                      title={t("action.delete")}
                      className="grid size-7 shrink-0 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-danger-subtle hover:text-danger"
                    >
                      <Icon as={IconTrash} size={14} />
                    </button>
                  </div>
                </Td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <LoginBinding items={items} />

      {creating && <CreateDialog onClose={() => setCreating(false)} />}
      {versions && <VersionsDialog item={versions} onClose={() => setVersions(null)} />}

      {deleting && (
        <ConfirmDialog
          title={t("microfrontends.deleteTitle", { name: deleting.name || deleting.url })}
          description={t("microfrontends.deleteDescription")}
          confirmLabel={t("action.delete")}
          busy={remove.isPending}
          onClose={() => setDeleting(null)}
          onConfirm={() => remove.mutate(deleting, { onSuccess: () => setDeleting(null) })}
        />
      )}
    </div>
  );
}

/**
 * Подмена экрана входа: какой микрофронтенд показывать вместо нашей
 * формы логина.
 *
 * Привязка идёт к ПОДДОМЕНУ, а не к проекту — так устроена запись
 * (`ProjectLoginMicroFrontend`), и у проекта с несколькими адресами
 * подменяется вход на конкретном. Поле подставлено адресом, с которого
 * открыта админка: настраивают обычно его.
 *
 * Стоит внизу раздела, а не в настройках проекта: выбирают здесь
 * из того же списка, что выше, и без него выбор был бы вслепую.
 */
function LoginBinding({ items }: { items: ManagedMicrofrontend[] }) {
  const { t } = useTranslation();
  const [typed, setTyped] = useState(() => loginSubdomain() || window.location.hostname);
  /*
   * Запрос — по отложенному значению: адрес набирают руками, а у чтения
   * привязки `staleTime: Infinity`, то есть каждый огрызок «a», «ap»,
   * «app» уехал бы в сеть и остался в кэше навсегда.
   *
   * Привязка и адрес берутся из ОДНОГО значения: отправить `id` записи
   * прежнего поддомена вместе с новым адресом значит перевесить чужую
   * привязку.
   */
  const subdomain = useDeferredValue(typed);
  const { binding } = useLoginMicrofront(subdomain);
  const bind = useBindLoginMicrofront();

  return (
    <div className="shrink-0 border-t border-border px-4 py-3">
      <p className="text-sm font-medium text-fg">{t("microfrontends.loginTitle")}</p>
      <p className="mt-0.5 mb-2 text-xs text-fg-subtle">{t("microfrontends.loginHint")}</p>

      <div className="flex flex-wrap items-end gap-2">
        <div className="w-64">
          <Input
            value={typed}
            onChange={(event) => setTyped(event.target.value.trim())}
            placeholder={t("microfrontends.loginSubdomain")}
            aria-label={t("microfrontends.loginSubdomain")}
            className="h-7 text-sm"
          />
        </div>

        <select
          value={binding?.microfrontId ?? ""}
          disabled={!subdomain || bind.isPending}
          onChange={(event) =>
            bind.mutate({ binding, microfrontId: event.target.value, subdomain })
          }
          aria-label={t("microfrontends.title")}
          className="h-7 min-w-48 rounded-md border border-border-strong bg-surface px-1.5 text-sm text-fg outline-none focus:border-accent disabled:opacity-40"
        >
          {/* Пустой выбор — это и есть «отвязать»: своей ручки снятия
              у бэкенда нет, снимается пустым `microfront_id`. */}
          <option value="">{t("microfrontends.loginNone")}</option>
          {items.map((item) => (
            <option key={item.id} value={item.id}>
              {item.name || item.url}
            </option>
          ))}
        </select>
      </div>
    </div>
  );
}

/**
 * Публикация в master и слежение за сборкой.
 *
 * Кнопки нет у микрофронтендов, заведённых руками: публикация требует
 * идентификатор проекта MCP, а его ставит только генератор — без него
 * ручка отвечает отказом (см. api/manage). Пустое место честнее
 * кнопки, которая всегда возвращает 400.
 */
function PromoteButton({ item }: { item: ManagedMicrofrontend }) {
  const { t } = useTranslation();
  const promote = usePromoteMicrofrontend();
  const changes = usePromoteChanges(item.repoId);
  const [pipelineId, setPipelineId] = useState("");
  const { status } = usePipelineStatus(item.repoId, pipelineId);

  if (!item.mcpProjectId || !item.repoId) return null;

  // Сборка идёт — показываем её состояние вместо кнопки: нажимать
  // второй раз, пока едет первая, незачем.
  if (pipelineId && !isPipelineDone(status)) {
    return <span className="text-2xs text-fg-muted">{status || t("common.loading")}</span>;
  }

  return (
    <Button
      size="sm"
      variant="secondary"
      disabled={promote.isPending || changes.isLoading || !changes.hasChanges}
      /* Пока ответ едет, кнопка тоже выключена — но молча: «публиковать
         нечего» о неизвестном ещё состоянии было бы неправдой. */
      title={changes.isLoading || changes.hasChanges ? undefined : t("microfrontends.nothingToPublish")}
      onClick={() =>
        promote.mutate(item, {
          onSuccess: (data) => setPipelineId(data.pipeline_id ? String(data.pipeline_id) : ""),
        })
      }
    >
      {t("microfrontends.publish")}
    </Button>
  );
}

function CreateDialog({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation();
  const create = useCreateMicrofrontend();
  const [draft, setDraft] = useState({ name: "", path: "", description: "" });

  return (
    <Modal onClose={onClose}>
      <form
        className="flex w-full max-w-md flex-col gap-4 rounded-xl border border-border bg-surface p-5 shadow-modal"
        onSubmit={(event) => {
          event.preventDefault();
          create.mutate(draft, { onSuccess: onClose });
        }}
      >
        <h2 className="text-base font-semibold">{t("microfrontends.create")}</h2>

        <Field label={t("microfrontends.name")}>
          <Input
            autoFocus
            required
            value={draft.name}
            onChange={(event) => setDraft({ ...draft, name: event.target.value })}
          />
        </Field>

        <Field label={t("functions.path")} hint={t("microfrontends.pathHint")}>
          <Input
            required
            value={draft.path}
            onChange={(event) => setDraft({ ...draft, path: event.target.value })}
          />
        </Field>

        <Field label={t("functions.description")}>
          <Input
            value={draft.description}
            onChange={(event) => setDraft({ ...draft, description: event.target.value })}
          />
        </Field>

        <p className="text-2xs text-fg-subtle">{t("microfrontends.createHint")}</p>

        <div className="flex justify-end gap-2">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("action.cancel")}
          </Button>
          <Button
            type="submit"
            disabled={create.isPending || !draft.name.trim() || !draft.path.trim()}
          >
            {create.isPending ? t("common.saving") : t("action.create")}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

/**
 * Версии: коммиты ветки `u-gen` слева, снимок выбранной справа.
 *
 * Это история изменений ИЗ АДМИНКИ, а не история репозитория: сервис
 * отдаёт только коммиты, сделанные его собственным токеном, чужие
 * ручные отсеивает по почте автора.
 */
function VersionsDialog({
  item,
  onClose,
}: {
  item: ManagedMicrofrontend;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const { commits, isLoading, error } = useMicrofrontendCommits(item.repoId);
  const revert = useRevertMicrofrontend();

  const [opened, setOpened] = useState("");
  const files = useFilesAtCommit(item.repoId, opened);
  const [file, setFile] = useState("");

  const current = files.files.find((one) => one.path === file) ?? files.files[0];

  return (
    <Modal onClose={onClose}>
      <div className="flex h-[85vh] w-full max-w-5xl flex-col gap-3 rounded-xl border border-border bg-surface p-5 shadow-modal">
        <h2 className="text-base font-semibold">
          {t("microfrontends.versions")} · {item.name || item.url}
        </h2>

        <div className="flex min-h-0 flex-1 gap-3">
          <div className="flex w-72 shrink-0 flex-col overflow-y-auto rounded-md border border-border">
            {isLoading && <p className="p-3 text-xs text-fg-subtle">{t("common.loading")}</p>}
            {!isLoading && !commits.length && (
              <p className="p-3 text-xs text-fg-subtle">{error ?? t("microfrontends.noVersions")}</p>
            )}

            {commits.map((commit) => (
              <button
                key={commit.sha}
                type="button"
                onClick={() => {
                  setOpened(commit.sha);
                  setFile("");
                }}
                className={`flex flex-col gap-0.5 border-b border-border px-3 py-2 text-left transition-colors ${
                  commit.sha === opened ? "bg-accent-subtle" : "hover:bg-surface-hover"
                }`}
              >
                <span className="truncate text-xs text-fg">
                  {commit.title || commit.shortSha}
                </span>
                <span className="truncate text-2xs text-fg-subtle">
                  {commit.shortSha} · {commit.date}
                </span>
              </button>
            ))}
          </div>

          <div className="flex min-w-0 flex-1 flex-col gap-2">
            {!opened && (
              <p className="text-xs text-fg-subtle">{t("microfrontends.pickVersion")}</p>
            )}

            {opened && (
              <>
                <div className="flex items-center gap-2">
                  <select
                    value={current?.path ?? ""}
                    onChange={(event) => setFile(event.target.value)}
                    aria-label={t("functions.code")}
                    className="h-7 min-w-0 flex-1 rounded-md border border-border-strong bg-surface px-1.5 font-mono text-2xs text-fg outline-none focus:border-accent"
                  >
                    {files.files.map((one) => (
                      <option key={one.path} value={one.path}>
                        {one.path}
                      </option>
                    ))}
                  </select>

                  {/* Откат кладёт снимок в `u-gen`. Живым он станет
                      только после публикации — так и написано. */}
                  <Button
                    size="sm"
                    variant="secondary"
                    disabled={revert.isPending}
                    onClick={() => revert.mutate({ repoId: item.repoId, sha: opened, id: item.id })}
                  >
                    {t("microfrontends.revert")}
                  </Button>
                </div>

                {files.isLoading && <p className="text-xs text-fg-subtle">{t("common.loading")}</p>}

                <pre className="min-h-0 flex-1 overflow-auto rounded-md bg-surface-hover p-3 font-mono text-xs text-fg">
                  {files.error ?? current?.content}
                </pre>

                <p className="text-2xs text-fg-subtle">{t("microfrontends.revertHint")}</p>
              </>
            )}
          </div>
        </div>

        <div className="flex justify-end">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("action.close")}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
