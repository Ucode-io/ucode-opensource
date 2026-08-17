import { useState } from "react";
import {
  IconBolt,
  IconChevronLeft,
  IconLoader2,
  IconPlus,
  IconSettings,
  IconTrash,
} from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { IconPicker } from "@/features/icons";
import { toast } from "@/shared/lib/toast";
import { Checkbox } from "@/shared/ui/checkbox";
import { CommitInput } from "@/shared/ui/commit-input";
import { DynamicIcon } from "@/shared/ui/dynamic-icon";
import { Icon } from "@/shared/ui/icon";
import { Select } from "@/shared/ui/input";
import { Popover, PopoverItem, PopoverSeparator } from "@/shared/ui/popover";
import { SelectMenu } from "@/shared/ui/select-menu";
import { ToolButton } from "@/shared/ui/tool-button";
import { useFunctions } from "../api/functions";
import {
  ACTION_METHODS,
  ACTION_TYPES,
  EMPTY_ACTION_DRAFT,
  toDraft,
  useActions,
  useCreateAction,
  useDeleteAction,
  useRunAction,
  useUpdateAction,
  type Action,
  type ActionDraft,
} from "../api/actions";
import { localized } from "../model/types";

/**
 * Действия таблицы: функции проекта, которые запускают над отмеченными
 * строками. Кнопка-молния — та же, что в старой админке.
 *
 * Список и запуск в одной панели, а не в модальном окне со своей
 * таблицей: действий у таблицы единицы, и всё, что о них нужно знать
 * при запуске, — подпись и значок. Настройка открывается страницей
 * внутри той же панели, как и у view.
 *
 * Запуск требует отмеченных строк: действие получает их guid'ы, и без
 * выделения оно сработало бы вхолостую. Об этом сказано в панели,
 * а не выяснено после нажатия.
 *
 * Чего здесь нет: `attributes.additional_parameters` — набор значений,
 * которые уезжают в функцию вместе со строками. Их смысл задаётся
 * графом связей таблицы (TABLE/OBJECTID/HARDCODE плюс слаг чужой
 * таблицы), и половина формы старой админки — это выбор из связей.
 * Настройка сохраняется нетронутой: тело правки собирается поверх
 * ответа.
 */
export function TableActions({
  tableSlug,
  language,
  languages,
  selected,
  canEdit,
}: {
  tableSlug: string;
  /** Язык ДАННЫХ: на нём подписаны действия. */
  language: string;
  languages: { code: string; nativeName: string }[];
  /** guid отмеченных строк: над ними запускается действие. */
  selected: string[];
  /** Право настраивать таблицу. Без него панель только запускает. */
  canEdit: boolean;
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  // Запрос уходит, только когда панель раскрыли: у большинства таблиц
  // действий нет вовсе, а список стоит запроса на каждую загрузку.
  const { actions } = useActions(tableSlug, open);

  return (
    <Popover
      align="end"
      trigger={({ open: shown, toggle }) => (
        <ToolButton
          icon={IconBolt}
          label={t("actions.title")}
          open={shown}
          on={selected.length > 0 && actions.length > 0}
          onClick={() => {
            setOpen(true);
            toggle();
          }}
        />
      )}
    >
      {(close) => (
        <Panel
          tableSlug={tableSlug}
          language={language}
          languages={languages}
          selected={selected}
          canEdit={canEdit}
          actions={actions}
          close={close}
        />
      )}
    </Popover>
  );
}

/** Открытая страница: список или настройка одного действия. */
type Page = { action: Action | null } | null;

function Panel({
  tableSlug,
  language,
  languages,
  selected,
  canEdit,
  actions,
  close,
}: {
  tableSlug: string;
  language: string;
  languages: { code: string; nativeName: string }[];
  selected: string[];
  canEdit: boolean;
  actions: Action[];
  close: () => void;
}) {
  const { t } = useTranslation();
  const [page, setPage] = useState<Page>(null);
  const run = useRunAction(tableSlug);

  if (page) {
    return (
      <ActionForm
        tableSlug={tableSlug}
        languages={languages}
        action={page.action}
        onBack={() => setPage(null)}
        onDone={() => setPage(null)}
      />
    );
  }

  /*
   * Выключенные действия в списке не показываются вовсе: `disable` —
   * это «не предлагать», а не «предложить серым».
   */
  const runnable = actions.filter((action) => !action.disabled);

  return (
    <div className="w-72">
      <p className="px-2 py-1.5 text-2xs text-fg-subtle">
        {selected.length
          ? t("actions.willRun", { count: selected.length })
          : t("actions.selectRows")}
      </p>

      {runnable.map((action) => (
        <div key={action.id} className="flex items-center">
          <PopoverItem
            icon={
              <DynamicIcon
                name={action.icon}
                size={16}
                fallback={<Icon as={IconBolt} size={16} className="shrink-0 text-fg-muted" />}
              />
            }
            onClick={() => {
              if (run.isPending) return;

              /*
               * Без выделения запускать нечего: строки — это и есть
               * аргумент действия. Строка при этом остаётся кликабельной
               * и отвечает словами: щелчок, после которого ничего
               * не происходит и никто ничего не сказал, читается
               * как поломка.
               */
              if (!selected.length) {
                toast.error(t("actions.selectRows"));
                return;
              }

              run.mutate({ action, guids: selected });
              close();
            }}
          >
            <span className={selected.length ? "" : "text-fg-subtle"}>
              {localized(action.labels, language, action.label) || t("actions.untitled")}
            </span>
          </PopoverItem>

          {canEdit && (
            <button
              type="button"
              onClick={() => setPage({ action })}
              aria-label={t("actions.settings")}
              title={t("actions.settings")}
              className="grid size-7 shrink-0 place-items-center rounded text-fg-subtle transition-colors hover:bg-surface-hover hover:text-fg"
            >
              <Icon as={IconSettings} size={14} />
            </button>
          )}
        </div>
      ))}

      {!runnable.length && (
        <p className="px-2 py-2 text-xs text-fg-subtle">{t("actions.empty")}</p>
      )}

      {canEdit && (
        <>
          <PopoverSeparator />
          <PopoverItem
            icon={<Icon as={IconPlus} size={16} className="shrink-0 text-fg-muted" />}
            onClick={() => setPage({ action: null })}
          >
            {t("actions.create")}
          </PopoverItem>
        </>
      )}
    </div>
  );
}

/**
 * Настройка действия. Правки уходят кнопкой, а не по одной: у нового
 * действия сохранять нечего, пока не выбрана функция, — а значит
 * и у существующего форма обязана вести себя так же.
 */
function ActionForm({
  tableSlug,
  languages,
  action,
  onBack,
  onDone,
}: {
  tableSlug: string;
  languages: { code: string; nativeName: string }[];
  /** null — новое действие. */
  action: Action | null;
  onBack: () => void;
  onDone: () => void;
}) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState<ActionDraft>(() =>
    action ? toDraft(action) : EMPTY_ACTION_DRAFT,
  );
  const [search, setSearch] = useState("");

  const { functions, isLoading } = useFunctions();
  const create = useCreateAction(tableSlug);
  const update = useUpdateAction(tableSlug);
  const remove = useDeleteAction(tableSlug);

  const patch = (next: Partial<ActionDraft>) => setDraft((value) => ({ ...value, ...next }));
  const busy = create.isPending || update.isPending || remove.isPending;
  // Без функции звать нечего: действие без неё — строка в списке,
  // которая ничего не делает.
  const ready = Boolean(draft.functionId);

  const submit = () => {
    if (!ready || busy) return;

    if (action) update.mutate({ action, draft }, { onSuccess: onDone });
    else create.mutate(draft, { onSuccess: onDone });
  };

  const matching = functions.filter((item) =>
    item.name.toLowerCase().includes(search.trim().toLowerCase()),
  );

  return (
    <div className="flex max-h-[70vh] w-80 flex-col">
      <div className="flex h-8 shrink-0 items-center gap-1 px-1">
        <button
          type="button"
          onClick={onBack}
          aria-label={t("action.back")}
          className="grid size-6 shrink-0 place-items-center rounded text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
        >
          <Icon as={IconChevronLeft} size={16} />
        </button>
        <span className="flex-1 truncate px-1 text-xs font-medium text-fg-muted">
          {t(action ? "actions.edit" : "actions.create")}
        </span>
        {busy && <Icon as={IconLoader2} size={12} className="shrink-0 animate-spin text-fg-subtle" />}
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto p-1">
        <div className="mb-2 flex flex-col gap-1">
          {languages.map((item) => (
            <label key={item.code} className="flex flex-col gap-0.5">
              <span className="px-0.5 text-2xs text-fg-subtle">{item.nativeName}</span>
              <CommitInput
                value={draft.labels[item.code] ?? ""}
                placeholder={t("actions.label")}
                label={t("actions.label")}
                allowEmpty
                onCommit={(label) => patch({ labels: { ...draft.labels, [item.code]: label } })}
              />
            </label>
          ))}
        </div>

        {/* Значок рисуется в списке действий рядом с подписью: без него
            все строки выглядят одинаково, а выбирают из них на бегу. */}
        <label className="mb-2 flex flex-col gap-0.5">
          <span className="px-0.5 text-2xs text-fg-muted">{t("actions.icon")}</span>
          <IconPicker value={draft.icon} type="ACTION" onChange={(icon) => patch({ icon })} />
        </label>

        {/* Функция — это и есть действие: остальное только оформление. */}
        <SelectMenu
          label={t("actions.function")}
          placeholder={t("button.noFunction")}
          searchPlaceholder={t("actions.searchFunction")}
          emptyText={t("button.noFunctions")}
          items={matching.map((item) => ({ value: item.id, label: item.name }))}
          selected={new Set(draft.functionId ? [draft.functionId] : [])}
          search={search}
          loading={isLoading}
          onSearch={setSearch}
          onLoadMore={() => {}}
          onPick={(functionId) => patch({ functionId })}
        />

        <label className="mt-2 flex flex-col gap-0.5">
          <span className="px-0.5 text-2xs text-fg-muted">{t("actions.type")}</span>
          <Select
            value={draft.actionType}
            onChange={(event) => patch({ actionType: event.target.value })}
          >
            {ACTION_TYPES.map((type) => (
              <option key={type} value={type}>
                {type}
              </option>
            ))}
          </Select>
        </label>

        <label className="mt-2 flex flex-col gap-0.5">
          <span className="px-0.5 text-2xs text-fg-muted">{t("actions.method")}</span>
          <Select value={draft.method} onChange={(event) => patch({ method: event.target.value })}>
            {ACTION_METHODS.map((method) => (
              <option key={method} value={method}>
                {method}
              </option>
            ))}
          </Select>
        </label>

        <label className="mt-2 flex flex-col gap-0.5">
          <span className="px-0.5 text-2xs text-fg-muted">{t("actions.url")}</span>
          <CommitInput
            value={draft.url}
            placeholder="https://"
            label={t("actions.url")}
            allowEmpty
            onCommit={(url) => patch({ url })}
          />
          <span className="px-0.5 text-2xs text-fg-subtle">{t("actions.urlHint")}</span>
        </label>

        <label className="mt-2 flex cursor-pointer items-center gap-2 rounded-md px-1 py-1 transition-colors hover:bg-surface-hover">
          <Checkbox
            checked={draft.disabled}
            onChange={(event) => patch({ disabled: event.target.checked })}
          />
          <span className="text-sm text-fg">{t("actions.disabled")}</span>
        </label>
      </div>

      <div className="flex shrink-0 items-center gap-1 border-t border-border p-1">
        {action && (
          <button
            type="button"
            onClick={() => remove.mutate(action, { onSuccess: onDone })}
            aria-label={t("action.delete")}
            title={t("action.delete")}
            className="grid size-8 shrink-0 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-danger-subtle hover:text-danger"
          >
            <Icon as={IconTrash} size={16} />
          </button>
        )}

        <button
          type="button"
          onClick={submit}
          disabled={!ready || busy}
          className="h-8 flex-1 rounded-md bg-accent-solid text-sm font-medium text-accent-fg transition-opacity hover:opacity-90 disabled:opacity-50"
        >
          {t("action.save")}
        </button>
      </div>
    </div>
  );
}
