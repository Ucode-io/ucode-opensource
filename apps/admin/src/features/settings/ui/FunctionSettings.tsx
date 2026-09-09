import { useDeferredValue, useState } from "react";
import { IconCode, IconPencil, IconPlayerPlay, IconPlus, IconTrash } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";
import { Icon } from "@/shared/ui/icon";
import { Dropdown } from "@/shared/ui/dropdown";
import { Field, Input } from "@/shared/ui/input";
import { Modal } from "@/shared/ui/modal";
import {
  EMPTY_FUNCTION_DRAFT,
  FUNCTIONS_PAGE,
  FUNCTION_TYPES,
  useCreateFunction,
  useDeleteFunction,
  useFunctionCodebase,
  useProjectFunctions,
  useRunFunction,
  useUpdateFunction,
  type FunctionDraft,
  type ProjectFunction,
} from "../api/functions";
import { Empty, Pager, SectionHeader, Td, Th } from "./parts";

/**
 * Функции проекта: список, карточка, запуск и ПРОСМОТР кода.
 *
 * Редактора кода здесь нет, и это не упрощение, а следствие: ручки,
 * которая пишет файлы функции обратно, у бэкенда не существует —
 * `push-changes` есть только у микрофронтенда. Показывать поле ввода,
 * из которого нельзя сохранить, значит обещать то, чего не выполнить
 * (см. api/functions).
 *
 * Путь и тип правятся только при создании: путь — это адрес
 * развёрнутой функции и имя её репозитория, тип решает, из какого
 * шаблона репозиторий форкнут. Задним числом ни то, ни другое
 * не переиграть — строка в базе поменялась бы, а запущенное осталось
 * бы прежним.
 */
export function FunctionSettings() {
  const { t } = useTranslation();

  const [query, setQuery] = useState("");
  const search = useDeferredValue(query);
  const [page, setPage] = useState(1);

  const { functions, count, isLoading, error } = useProjectFunctions(search, page);
  const remove = useDeleteFunction();

  const [editing, setEditing] = useState<ProjectFunction | "new" | null>(null);
  const [deleting, setDeleting] = useState<ProjectFunction | null>(null);
  const [running, setRunning] = useState<ProjectFunction | null>(null);
  const [viewing, setViewing] = useState<ProjectFunction | null>(null);

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <SectionHeader title={t("functions.title")} hint={t("functions.hint")}>
        <div className="w-48">
          <Input
            value={query}
            onChange={(event) => {
              setQuery(event.target.value);
              setPage(1);
            }}
            placeholder={t("functions.search")}
            aria-label={t("functions.search")}
            className="h-7 text-sm"
          />
        </div>

        <Button size="sm" onClick={() => setEditing("new")}>
          <Icon as={IconPlus} size={14} />
          {t("functions.create")}
        </Button>
      </SectionHeader>

      <div className="min-h-0 flex-1 overflow-auto">
        <table className="w-full border-separate border-spacing-0">
          <thead>
            <tr>
              <Th>{t("functions.name")}</Th>
              <Th>{t("functions.path")}</Th>
              <Th className="w-28">{t("functions.type")}</Th>
              <Th className="w-28">{t("functions.branch")}</Th>
              <Th className="w-40">{t("functions.pipeline")}</Th>
              <Th className="w-28" />
            </tr>
          </thead>

          <tbody>
            {isLoading && <Empty text={t("common.loading")} colSpan={6} />}
            {!isLoading && !functions.length && (
              <Empty text={error ?? t("functions.empty")} colSpan={6} />
            )}

            {functions.map((item) => (
              <tr key={item.id} className="hover:bg-surface-hover">
                <Td>{item.name || t("functions.untitled")}</Td>
                <Td className="font-mono text-xs text-fg-muted">{item.path}</Td>
                <Td className="text-fg-muted">{item.type}</Td>
                <Td className="text-fg-muted">{item.branch}</Td>
                <Td>
                  <PipelineStatus item={item} />
                </Td>
                <Td>
                  <div className="flex items-center justify-end gap-0.5">
                    {/* Просмотр кода — только там, где код есть. У WORKFLOW
                        репозитория не заводят вовсе, и ручка ответила бы
                        «function has no linked gitlab repository». */}
                    {item.type !== "WORKFLOW" && (
                      <RowButton
                        icon={IconCode}
                        label={t("functions.code")}
                        onClick={() => setViewing(item)}
                      />
                    )}
                    <RowButton
                      icon={IconPlayerPlay}
                      label={t("functions.run")}
                      onClick={() => setRunning(item)}
                    />
                    <RowButton
                      icon={IconPencil}
                      label={t("action.edit")}
                      onClick={() => setEditing(item)}
                    />
                    <RowButton
                      icon={IconTrash}
                      label={t("action.delete")}
                      danger
                      onClick={() => setDeleting(item)}
                    />
                  </div>
                </Td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <Pager page={page} total={count} limit={FUNCTIONS_PAGE} onPage={setPage} />

      {editing && (
        <FunctionDialog
          item={editing === "new" ? null : editing}
          onClose={() => setEditing(null)}
        />
      )}

      {running && <RunDialog item={running} onClose={() => setRunning(null)} />}
      {viewing && <CodeDialog item={viewing} onClose={() => setViewing(null)} />}

      {deleting && (
        <ConfirmDialog
          title={t("functions.deleteTitle", { name: deleting.name || deleting.path })}
          description={t("functions.deleteDescription")}
          confirmLabel={t("action.delete")}
          busy={remove.isPending}
          onClose={() => setDeleting(null)}
          onConfirm={() => remove.mutate(deleting, { onSuccess: () => setDeleting(null) })}
        />
      )}
    </div>
  );
}

function RowButton({
  icon,
  label,
  danger,
  onClick,
}: {
  icon: typeof IconCode;
  label: string;
  danger?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      title={label}
      className={`grid size-7 shrink-0 place-items-center rounded-md text-fg-subtle transition-colors ${
        danger ? "hover:bg-danger-subtle hover:text-danger" : "hover:bg-surface-active hover:text-fg"
      }`}
    >
      <Icon as={icon} size={14} />
    </button>
  );
}

/**
 * Статус сборки. Провал показывается вместе с причиной: `error_message`
 * — это то, что сказал CI, и без него «failed» отправляет искать логи
 * в GitLab, куда у админа проекта доступа нет.
 */
function PipelineStatus({ item }: { item: ProjectFunction }) {
  const failed = /fail|error|cancel/i.test(item.pipelineStatus);

  if (!item.pipelineStatus) return <span className="text-2xs text-fg-subtle">—</span>;

  return (
    <span
      title={item.errorMessage || undefined}
      className={`text-2xs ${failed ? "text-danger" : "text-fg-muted"}`}
    >
      {item.pipelineStatus}
      {failed && item.errorMessage && (
        <span className="block truncate text-fg-subtle">{item.errorMessage}</span>
      )}
    </span>
  );
}

/** Заведение и правка. Отличаются тем, что путь и тип задаются один раз. */
function FunctionDialog({ item, onClose }: { item: ProjectFunction | null; onClose: () => void }) {
  const { t } = useTranslation();
  const create = useCreateFunction();
  const update = useUpdateFunction();

  const [draft, setDraft] = useState<FunctionDraft>(() =>
    item
      ? { name: item.name, path: item.path, description: item.description, type: item.type }
      : EMPTY_FUNCTION_DRAFT,
  );

  const busy = create.isPending || update.isPending;
  const put = (patch: Partial<FunctionDraft>) => setDraft((value) => ({ ...value, ...patch }));

  return (
    <Modal onClose={onClose}>
      <form
        className="flex w-full max-w-md flex-col gap-4 rounded-xl border border-border bg-surface p-5 shadow-modal"
        onSubmit={(event) => {
          event.preventDefault();
          if (busy) return;

          if (item) update.mutate({ item, draft }, { onSuccess: onClose });
          else create.mutate(draft, { onSuccess: onClose });
        }}
      >
        <h2 className="text-base font-semibold">
          {t(item ? "functions.editTitle" : "functions.create")}
        </h2>

        <Field label={t("functions.name")}>
          <Input
            autoFocus
            required
            value={draft.name}
            onChange={(event) => put({ name: event.target.value })}
          />
        </Field>

        <Field label={t("functions.path")} {...(item ? {} : { hint: t("functions.pathHint") })}>
          <Input
            required
            readOnly={Boolean(item)}
            value={draft.path}
            onChange={(event) => put({ path: event.target.value })}
          />
        </Field>

        <Field label={t("functions.type")} {...(item ? { hint: t("functions.typeFixed") } : {})}>
          <Dropdown
            value={draft.type}
            disabled={Boolean(item)}
            items={FUNCTION_TYPES.map((type) => ({ value: type, label: type }))}
            onChange={(type) => put({ type })}
          />
        </Field>

        <Field label={t("functions.description")}>
          <Input
            value={draft.description}
            onChange={(event) => put({ description: event.target.value })}
          />
        </Field>

        <div className="flex justify-end gap-2">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("action.cancel")}
          </Button>
          <Button type="submit" disabled={busy || !draft.name.trim() || !draft.path.trim()}>
            {busy ? t("common.saving") : t(item ? "action.save" : "action.create")}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

/**
 * Запуск: тело запроса и ответ функции рядом.
 *
 * Ответ показывается целиком и как есть — это единственное место
 * в админке, где видно, что функция вернула на самом деле: `/v1/invoke_function`,
 * которой её зовут действия таблицы, отвечает пустотой всегда
 * (docs/backend-notes.md, «Функции»).
 */
function RunDialog({ item, onClose }: { item: ProjectFunction; onClose: () => void }) {
  const { t } = useTranslation();
  const run = useRunFunction();
  const [body, setBody] = useState("{}");
  const [invalid, setInvalid] = useState(false);

  const submit = () => {
    let parsed: unknown;
    try {
      parsed = JSON.parse(body || "{}");
    } catch {
      setInvalid(true);
      return;
    }

    setInvalid(false);
    run.mutate({ id: item.id, body: parsed });
  };

  return (
    <Modal onClose={onClose}>
      <div className="flex h-[80vh] w-full max-w-2xl flex-col gap-3 rounded-xl border border-border bg-surface p-5 shadow-modal">
        <h2 className="text-base font-semibold">
          {t("functions.run")} · {item.name || item.path}
        </h2>

        <label className="flex min-h-0 flex-1 flex-col gap-1">
          <span className="text-2xs text-fg-muted">{t("functions.runBody")}</span>
          <textarea
            value={body}
            onChange={(event) => setBody(event.target.value)}
            spellCheck={false}
            className="min-h-0 flex-1 resize-none rounded-md border border-border-strong bg-surface p-2 font-mono text-xs text-fg outline-none focus:border-accent"
          />
        </label>

        {invalid && (
          <p role="alert" className="text-2xs text-danger">
            {t("cell.jsonInvalid")}
          </p>
        )}

        {run.data !== undefined && (
          <div className="flex min-h-0 flex-1 flex-col gap-1">
            <span className="text-2xs text-fg-muted">{t("functions.runResult")}</span>
            <pre className="min-h-0 flex-1 overflow-auto rounded-md bg-surface-hover p-2 font-mono text-xs text-fg">
              {JSON.stringify(run.data, null, 2)}
            </pre>
          </div>
        )}

        <div className="flex justify-end gap-2">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("action.close")}
          </Button>
          <Button type="button" disabled={run.isPending} onClick={submit}>
            {run.isPending ? t("common.loading") : t("functions.run")}
          </Button>
        </div>
      </div>
    </Modal>
  );
}

/**
 * Исходники: список файлов слева, содержимое справа.
 *
 * Подсветки синтаксиса нет намеренно: она стоит отдельной библиотеки
 * с грамматиками на каждый язык, а читают здесь обычно один файл и
 * чтобы понять, что вообще развёрнуто.
 */
function CodeDialog({ item, onClose }: { item: ProjectFunction; onClose: () => void }) {
  const { t } = useTranslation();
  const { files, isLoading, error } = useFunctionCodebase(item.id);
  const [opened, setOpened] = useState("");

  const current = files.find((file) => file.path === opened) ?? files[0];

  return (
    <Modal onClose={onClose}>
      <div className="flex h-[85vh] w-full max-w-5xl flex-col gap-3 rounded-xl border border-border bg-surface p-5 shadow-modal">
        <h2 className="text-base font-semibold">
          {t("functions.code")} · {item.name || item.path}
        </h2>
        <p className="text-2xs text-fg-subtle">{t("functions.codeHint")}</p>

        {isLoading && <p className="text-sm text-fg-muted">{t("common.loading")}</p>}
        {!isLoading && !files.length && (
          <p className="text-sm text-fg-muted">{error ?? t("functions.codeEmpty")}</p>
        )}

        {Boolean(files.length) && (
          <div className="flex min-h-0 flex-1 gap-3">
            <nav className="w-64 shrink-0 overflow-y-auto rounded-md border border-border p-1">
              {files.map((file) => (
                <button
                  key={file.path}
                  type="button"
                  onClick={() => setOpened(file.path)}
                  className={`block w-full truncate rounded px-2 py-1 text-left font-mono text-2xs transition-colors ${
                    file.path === current?.path
                      ? "bg-accent-subtle text-accent-text"
                      : "text-fg-muted hover:bg-surface-hover"
                  }`}
                >
                  {file.path}
                </button>
              ))}
            </nav>

            <pre className="min-w-0 flex-1 overflow-auto rounded-md bg-surface-hover p-3 font-mono text-xs text-fg">
              {current?.content}
            </pre>
          </div>
        )}

        <div className="flex justify-end">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("action.close")}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
