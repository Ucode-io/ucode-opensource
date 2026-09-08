import { useDeferredValue, useEffect, useRef, useState } from "react";
import { IconCheck, IconPlus, IconTrash } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { Checkbox } from "@/shared/ui/checkbox";
import { Chip } from "@/shared/ui/chip";
import { Icon } from "@/shared/ui/icon";
import { Dropdown } from "@/shared/ui/dropdown";
import { Input } from "@/shared/ui/input";
import type { TranslationKey } from "@/shared/lib/i18n";
import { useRelationRows } from "../api/relation-rows";
import { useTableSchema } from "../api/schema";
import {
  AGGREGATES,
  filterParts,
  tableFromSlug,
  type Aggregate,
  type AggregateFilter,
  type FieldDraft,
} from "../model/field-draft";
import { localized, type Field, type Relation } from "../model/types";

/**
 * Настройки полей-формул. Слово одно, поля два, и общего у них ничего:
 *
 *   FORMULA_FRONTEND  выражение по полям ЭТОЙ строки, считает браузер
 *                     на каждый показ ячейки (см. features/item)
 *   FORMULA           «сумма поля X по связанным строкам таблицы Y»,
 *                     считает бэкенд, в строке лежит готовое число
 *
 * Поэтому и экрана два. Общая часть — только место в панели поля.
 */
export function FormulaSettings({
  draft,
  fields,
  relations,
  language,
  onChange,
}: {
  draft: FieldDraft;
  /** Поля ЭТОЙ таблицы: из них собирается выражение. */
  fields: Field[];
  /** Связи ЭТОЙ таблицы: по ним выбирается таблица агрегата. */
  relations: Relation[];
  language: string;
  onChange: (next: Partial<FieldDraft>) => void;
}) {
  /*
   * Выражение и шаблон строки — разные вещи, но правятся одинаково:
   * текст, в который подставляются слаги полей этой же строки. Разница
   * в том, кто и когда его читает, и она сказана подписью.
   */
  if (draft.type === "FORMULA_FRONTEND" || draft.type === "MANUAL_STRING") {
    return (
      <ExpressionEditor
        formula={draft.formula}
        fields={fields.filter((field) => field.slug !== draft.slug)}
        language={language}
        template={draft.type === "MANUAL_STRING"}
        onChange={(formula) => onChange({ formula })}
      />
    );
  }

  if (draft.type !== "FORMULA") return null;

  return (
    <AggregateEditor
      aggregate={draft.aggregate}
      relations={relations}
      language={language}
      onChange={(aggregate) => onChange({ aggregate })}
    />
  );
}

/**
 * Выражение и слаги полей под ним.
 *
 * Список полей не украшение: в выражении участвуют СЛАГИ, а человек
 * знает поля по подписям — «Цена» и `price_uzs` в голове не совпадают.
 * Клик вставляет слаг туда, где стоит курсор.
 *
 * `template` — это MANUAL_STRING: тот же текст со слагами, но не
 * выражение, а строка. Считает её бэкенд, и ровно один раз, при вставке
 * записи, — поэтому и подпись другая: «сумма пересчитается» и «номер
 * уже выдан» это разные обещания.
 */
function ExpressionEditor({
  formula,
  fields,
  language,
  template = false,
  onChange,
}: {
  formula: string;
  fields: Field[];
  language: string;
  template?: boolean;
  onChange: (formula: string) => void;
}) {
  const { t } = useTranslation();
  const area = useRef<HTMLTextAreaElement>(null);
  const label = t(template ? "formula.template" : "formula.expression");

  const insert = (slug: string) => {
    const element = area.current;
    const from = element?.selectionStart ?? formula.length;
    const to = element?.selectionEnd ?? from;

    onChange(`${formula.slice(0, from)}${slug}${formula.slice(to)}`);

    /*
     * Каретка — за вставленный слаг, и обязательно после перерисовки:
     * значение поля управляемое, и позиция, выставленная до неё,
     * сбрасывается в конец. Иначе вторая вставка уезжает не туда.
     */
    const at = from + slug.length;
    requestAnimationFrame(() => {
      element?.focus();
      element?.setSelectionRange(at, at);
    });
  };

  return (
    <div className="px-2 py-1">
      <span className="mb-0.5 block text-2xs text-fg-muted">{label}</span>

      <textarea
        ref={area}
        rows={3}
        value={formula}
        spellCheck={false}
        placeholder={template ? "INV-order_number/client_name" : "(price * count) * 1.12"}
        aria-label={label}
        onChange={(event) => onChange(event.target.value)}
        className="w-full resize-none rounded-md border border-border-strong bg-surface px-2 py-1.5 font-mono text-xs text-fg outline-none focus:border-accent"
      />

      {/* Обещание, которое стоит проговорить: у формулы значение живое,
          у шаблона — снимок. Правка шаблона старые строки не трогает. */}
      {template && (
        <span className="mt-1 block text-2xs text-fg-subtle">{t("formula.templateHint")}</span>
      )}

      {fields.length > 0 && (
        <>
          <span className="mt-1 mb-1 block text-2xs text-fg-muted">{t("formula.fields")}</span>

          <div className="flex flex-wrap gap-1">
            {fields.map((field) => (
              <button
                key={field.id}
                type="button"
                onClick={() => insert(field.slug)}
                title={localized(field.labels, language, field.label)}
                className="max-w-full rounded-sm transition-opacity hover:opacity-80"
              >
                <Chip>{field.slug}</Chip>
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  );
}

/**
 * Агрегат: пять настроек и отбор строк — ровно то же, что спрашивала
 * старая админка, потому что читает их тот же бэкенд.
 *
 * Таблица выбирается не из всех, а из СВЯЗАННЫХ: считать сумму по
 * таблице, с которой эта не связана, бэкенду не по чему — он идёт
 * по связи (`table_from` хранит и слаг, и её id одной строкой).
 */
function AggregateEditor({
  aggregate,
  relations,
  language,
  onChange,
}: {
  aggregate: Aggregate;
  relations: Relation[];
  language: string;
  onChange: (aggregate: Aggregate) => void;
}) {
  const { t } = useTranslation();
  const patch = (next: Partial<Aggregate>) => onChange({ ...aggregate, ...next });

  const slug = tableFromSlug(aggregate.tableFrom);
  /*
   * Поля чужой таблицы: и для «по какому полю считать», и для отбора.
   * Запрос ровно один и только когда таблицу выбрали — хук сам сидит
   * выключенным на пустом слаге.
   *
   * Настройки связей грузятся не все, а только тех, по которым уже
   * отбирают: в них лежат поля показа, без которых строку в списке
   * нечем подписать. Связей у чужой таблицы бывает полтора десятка,
   * и каждая — отдельный запрос.
   */
  const [needed, setNeeded] = useState<string[]>([]);
  const { schema } = useTableSchema(slug || undefined, needed);

  const relationIds = aggregate.filters
    .map((filter) => filterParts(filter.key).slug)
    .map((fieldSlug) => schema.fields.find((field) => field.slug === fieldSlug)?.relationId)
    .filter((id): id is string => Boolean(id));

  /*
   * Список меняется вслед за полями, а не одновременно с ними: сами
   * поля приезжают тем же хуком, и пока их нет, id связи неоткуда взять.
   * Сравнение по строке — чтобы не гонять эффект на новом массиве
   * с тем же содержимым.
   */
  const key = relationIds.join(",");
  useEffect(() => {
    setNeeded(key ? key.split(",") : []);
  }, [key]);

  const linked = relations.filter((relation) => relation.toSlug);
  const summable = schema.fields.filter((field) => field.type !== "LOOKUP");

  return (
    <div className="flex flex-col gap-1.5 px-2 py-1">
      <Labeled label={t("formula.aggregate")}>
        <Dropdown
          size="sm"
          value={aggregate.type}
          placeholder="—"
          items={AGGREGATES.map((type) => ({
            value: type,
            label: t(`formula.${type}` as TranslationKey),
          }))}
          onChange={(type) => patch({ type: type as Aggregate["type"] })}
        />
      </Labeled>

      <Labeled label={t("formula.table")}>
        <Dropdown
          size="sm"
          value={aggregate.tableFrom}
          placeholder="—"
          items={linked.map((relation) => ({
            value: `${relation.toSlug}#${relation.id}`,
            label: relation.toSlug,
          }))}
          // Поле и отбор считались по прежней таблице: в новой таких
          // слагов нет, и оставить их значит отправить отбор по полям,
          // которых там не существует.
          onChange={(tableFrom) => patch({ tableFrom, field: "", filters: [] })}
        />
      </Labeled>

      {slug && (
        <Labeled label={t("formula.field")}>
          <Dropdown
            size="sm"
            value={aggregate.field}
            placeholder="—"
            items={summable.map((field) => ({
              value: field.slug,
              label: localized(field.labels, language, field.label),
            }))}
            onChange={(field) => patch({ field })}
          />
        </Labeled>
      )}

      <Labeled label={t("formula.rounds")}>
        <Input
          value={aggregate.rounds}
          inputMode="numeric"
          placeholder="0"
          onChange={(event) => patch({ rounds: event.target.value.replace(/\D/g, "") })}
          className="h-7 px-1.5 text-xs tabular-nums"
        />
      </Labeled>

      {slug && (
        <FilterList
          filters={aggregate.filters}
          fields={schema.fields}
          relations={schema.relations}
          language={language}
          onChange={(filters) => patch({ filters })}
        />
      )}
    </div>
  );
}

/** Подпись над полем ввода в панели. Общая с настройками автозаполнения. */
export function Labeled({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="flex flex-col gap-0.5">
      <span className="text-2xs text-fg-muted">{label}</span>
      {children}
    </label>
  );
}

/**
 * По каким строкам считать.
 *
 * Отбирать можно не по всякому полю: бэкенд сравнивает значение
 * на равенство, и осмысленно это ровно у пяти типов — связь, выбор
 * и два булевых. Тот же список был и в старой админке.
 */
const FILTERABLE = new Set(["LOOKUP", "LOOKUPS", "MULTISELECT", "SWITCH", "CHECKBOX"]);

function FilterList({
  filters,
  fields,
  relations,
  language,
  onChange,
}: {
  filters: AggregateFilter[];
  /** Поля ЧУЖОЙ таблицы — той, по которой считаем. */
  fields: Field[];
  /** Её же связи: по ним ищутся строки для условия по связи. */
  relations: Relation[];
  language: string;
  onChange: (filters: AggregateFilter[]) => void;
}) {
  const { t } = useTranslation();
  const usable = fields.filter((field) => FILTERABLE.has(field.type));

  const patch = (index: number, next: Partial<AggregateFilter>) =>
    onChange(filters.map((filter, i) => (i === index ? { ...filter, ...next } : filter)));

  return (
    <div className="flex flex-col gap-1">
      <div className="flex h-5 items-center gap-1">
        <span className="flex-1 text-2xs text-fg-muted">{t("formula.filters")}</span>

        <button
          type="button"
          onClick={() => onChange([...filters, { key: "", value: null }])}
          disabled={!usable.length}
          aria-label={t("formula.addFilter")}
          title={t("formula.addFilter")}
          className="grid size-5 shrink-0 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-surface-hover hover:text-fg disabled:opacity-40"
        >
          <Icon as={IconPlus} size={14} />
        </button>
      </div>

      {filters.map((filter, index) => {
        const { slug } = filterParts(filter.key);
        const field = usable.find((item) => item.slug === slug);

        return (
          <div key={index} className="flex flex-col gap-1 rounded-md border border-border p-1">
            <div className="flex items-center gap-1">
              <Dropdown
                size="sm"
                value={filter.key}
                placeholder="—"
                className="min-w-0 flex-1"
                items={usable.map((item) => ({
                  value: filterKey(item),
                  label: localized(item.labels, language, item.label),
                }))}
                // Ключ составной — «слаг#ТИП#таблица»: в нём и лежит всё,
                // что нужно и нам, и бэкенду, чтобы понять условие.
                onChange={(key) => patch(index, { key, value: null })}
              />

              <button
                type="button"
                onClick={() => onChange(filters.filter((_, i) => i !== index))}
                aria-label={t("action.delete")}
                className="grid size-7 shrink-0 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-danger-subtle hover:text-danger"
              >
                <Icon as={IconTrash} size={14} />
              </button>
            </div>

            {field && (
              <FilterValue
                field={field}
                relation={relations.find((item) => item.id === field.relationId)}
                value={filter.value}
                language={language}
                onChange={(value) => patch(index, { value })}
              />
            )}
          </div>
        );
      })}
    </div>
  );
}

/** Ключ условия ровно в том виде, в каком его читает бэкенд. */
function filterKey(field: Field): string {
  const table = field.raw["table_slug"];
  return `${field.slug}#${field.type}#${typeof table === "string" ? table : ""}`;
}

/**
 * Значение условия. Форма зависит от типа поля, и это не украшение:
 * у выбора это список вариантов, у переключателя — да/нет, у связи —
 * строки чужой таблицы.
 */
function FilterValue({
  field,
  relation,
  value,
  language,
  onChange,
}: {
  field: Field;
  /** Связь поля: по ней ищутся строки. Нет — остаётся ввод guid'ов. */
  relation: Relation | undefined;
  value: unknown;
  language: string;
  onChange: (value: unknown) => void;
}) {
  const { t } = useTranslation();

  if (field.type === "SWITCH" || field.type === "CHECKBOX") {
    return (
      <label className="flex h-7 cursor-pointer items-center gap-2 px-1">
        <Checkbox checked={value === true} onChange={(event) => onChange(event.target.checked)} />
        <span className="truncate text-xs text-fg">{t("action.yes")}</span>
      </label>
    );
  }

  if (field.type === "MULTISELECT") {
    const picked = asList(value);

    return (
      <div className="flex flex-wrap gap-1 p-0.5">
        {[...field.options.values()].map((option) => {
          const on = picked.includes(option.value);

          return (
            <button
              key={option.value}
              type="button"
              onClick={() =>
                onChange(
                  on ? picked.filter((item) => item !== option.value) : [...picked, option.value],
                )
              }
              className={`rounded-sm transition-opacity ${on ? "" : "opacity-40"}`}
            >
              <Chip>{localized(option.labels, language, option.label || option.value)}</Chip>
            </button>
          );
        })}

        {field.options.size === 0 && (
          <span className="px-1 text-2xs text-fg-subtle">{t("table.noOptions")}</span>
        )}
      </div>
    );
  }

  /*
   * Связь: выбирают строки чужой таблицы, а уезжают их guid'ы.
   * Настройки связи ещё не приехали — остаётся ввод guid'ов: он честнее
   * пустого списка, из которого нечего выбрать, и им же правится
   * условие, заведённое когда-то руками.
   */
  if (relation?.toSlug) {
    return (
      <RelationPicker
        relation={relation}
        picked={asList(value)}
        onChange={(next) => onChange(next)}
      />
    );
  }

  return (
    <Input
      value={asList(value).join(", ")}
      placeholder={t("formula.guids")}
      aria-label={t("formula.guids")}
      onChange={(event) =>
        onChange(
          event.target.value
            .split(",")
            .map((item) => item.trim())
            .filter(Boolean),
        )
      }
      className="h-7 px-1.5 font-mono text-2xs"
    />
  );
}

/**
 * Выбор строк чужой таблицы: поиск и список.
 *
 * Отмеченное показывается сверху, потому что найденное списком уезжает
 * из виду при следующем поиске, а условие собирают из нескольких строк.
 * Подпись отмеченной строки известна, только пока она в ответе; чего
 * нет — показываем guid'ом, а не выдумываем: условие было заведено
 * раньше, и строка могла с тех пор исчезнуть.
 */
function RelationPicker({
  relation,
  picked,
  onChange,
}: {
  relation: Relation;
  picked: string[];
  onChange: (guids: string[]) => void;
}) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");
  /*
   * Запрос отстаёт от ввода на кадр — иначе каждая буква идёт в чужую
   * таблицу. Тот же приём, что и в ячейке-связи: отменять нечего.
   */
  const search = useDeferredValue(query);

  const { rows } = useRelationRows({
    tableSlug: relation.toSlug,
    viewFields: relation.viewFields,
    search,
  });

  const labels = new Map(rows.map((row) => [row.guid, row.label]));
  const toggle = (guid: string) =>
    onChange(picked.includes(guid) ? picked.filter((item) => item !== guid) : [...picked, guid]);

  return (
    <div className="flex flex-col gap-1">
      {picked.length > 0 && (
        <div className="flex flex-wrap gap-1 px-0.5">
          {picked.map((guid) => (
            <button
              key={guid}
              type="button"
              onClick={() => toggle(guid)}
              title={t("cell.remove")}
              className="max-w-full rounded-sm transition-opacity hover:opacity-70"
            >
              <Chip>{labels.get(guid) || guid.slice(0, 8)}</Chip>
            </button>
          ))}
        </div>
      )}

      <Input
        value={query}
        placeholder={t("cell.searchRelation")}
        aria-label={t("cell.searchRelation")}
        onChange={(event) => setQuery(event.target.value)}
        className="h-7 px-1.5 text-xs"
      />

      <div className="max-h-32 overflow-y-auto">
        {rows.map((row) => (
          <button
            key={row.guid}
            type="button"
            onClick={() => toggle(row.guid)}
            className={`flex h-7 w-full items-center gap-1.5 rounded px-1.5 text-left text-xs transition-colors hover:bg-surface-hover ${
              picked.includes(row.guid) ? "text-fg" : "text-fg-muted"
            }`}
          >
            {/* Галка значком, а не флажком: строка списка — уже кнопка,
                а <input> внутри <button> это вложенный интерактив. */}
            <Icon
              as={IconCheck}
              size={14}
              className={`shrink-0 ${picked.includes(row.guid) ? "text-accent" : "opacity-0"}`}
            />
            <span className="truncate">{row.label || row.guid.slice(0, 8)}</span>
          </button>
        ))}

        {rows.length === 0 && (
          <p className="px-1.5 py-1 text-2xs text-fg-subtle">{t("table.noRows")}</p>
        )}
      </div>
    </div>
  );
}

function asList(value: unknown): string[] {
  if (Array.isArray(value)) return value.map(String);
  return value === null || value === undefined || value === "" ? [] : [String(value)];
}
