import { useState } from "react";
import { IconChevronDown, IconDots, IconTrash } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { localized, type Field } from "@/features/table";
import type { TranslationKey } from "@/shared/lib/i18n";
import { Checkbox } from "@/shared/ui/checkbox";
import { Icon } from "@/shared/ui/icon";
import { Popover, PopoverItem } from "@/shared/ui/popover";
import { filterKind, kindOfOperator, operatorsFor } from "../model/filter-kind";
import { isFilterSet, type Filter, type FilterOperator } from "../model/query";
import { fieldIcon } from "./field-icon";

/**
 * Один фильтр в подшапке: чип с названием поля, а внутри — условие
 * и его аргументы.
 *
 * Условие живёт в шапке поповера, рядом с названием поля, а не
 * подразумевается формой ввода: «содержит» и «равно» выглядят одинаково,
 * и без явной подписи невозможно понять, почему точный запрос ничего
 * не нашёл.
 */
export function FilterChip({
  field,
  language,
  filter,
  onChange,
  onRemove,
}: {
  field: Field;
  language: string;
  filter: Filter;
  onChange: (filter: Filter) => void;
  onRemove: () => void;
}) {
  const { t } = useTranslation();
  const kind = filterKind(field);
  const label = localized(field.labels, language, field.label);
  const set = isFilterSet(filter);

  if (!kind) return null;

  const operators = operatorsFor(kind);

  return (
    <Popover
      trigger={({ open, toggle }) => (
        <button
          type="button"
          onClick={toggle}
          aria-expanded={open}
          className={`flex h-7 max-w-72 items-center gap-1.5 rounded-full border px-2 text-xs transition-colors ${
            set
              ? "border-accent bg-accent-subtle text-accent-text"
              : "border-border-strong text-fg-muted hover:bg-surface-hover hover:text-fg"
          }`}
        >
          <Icon as={fieldIcon(field.type)} size={14} />
          <span className="truncate font-medium">{label}</span>
          {set && <span className="truncate">: {summary(filter, field, language, t)}</span>}
          <Icon as={IconChevronDown} size={14} className="opacity-60" />
        </button>
      )}
    >
      {(close) => (
        <div className="w-64">
          <div className="flex h-8 items-center gap-1 px-1">
            <span className="shrink-0 text-xs text-fg-muted">{label}</span>

            <OperatorPicker
              operators={operators}
              value={filter.op}
              // Аргументы старого условия новому не подходят: список
              // вариантов не превратится в дату. Сбрасываем — пустой
              // фильтр честнее случайно уцелевшего значения.
              onChange={(op) => onChange({ op, values: [] })}
            />

            {/* Распорка, а не ml-auto на кнопке: кнопка лежит внутри
                обёртки Popover, и отступ достался бы ей, а не обёртке. */}
            <span className="flex-1" />

            <Popover
              align="end"
              trigger={({ toggle }) => (
                <button
                  type="button"
                  onClick={toggle}
                  aria-label={t("table.filterActions")}
                  className="grid size-6 shrink-0 place-items-center rounded-md text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
                >
                  <Icon as={IconDots} size={14} />
                </button>
              )}
            >
              {() => (
                <PopoverItem
                  danger
                  icon={<Icon as={IconTrash} size={14} />}
                  onClick={() => {
                    onRemove();
                    close();
                  }}
                >
                  {t("table.removeFilter")}
                </PopoverItem>
              )}
            </Popover>
          </div>

          <div className="border-t border-border pt-1">
            <Values field={field} language={language} filter={filter} onChange={onChange} />
          </div>
        </div>
      )}
    </Popover>
  );
}

/** Стрелка появляется, только когда есть из чего выбирать. */
function OperatorPicker({
  operators,
  value,
  onChange,
}: {
  operators: FilterOperator[];
  value: FilterOperator;
  onChange: (op: FilterOperator) => void;
}) {
  const { t } = useTranslation();
  const name = (op: FilterOperator) => t(`filter.op.${op}` as TranslationKey);

  if (operators.length < 2) {
    return <span className="text-xs font-medium text-fg">{name(value)}</span>;
  }

  return (
    <Popover
      trigger={({ toggle }) => (
        <button
          type="button"
          onClick={toggle}
          className="flex h-6 items-center gap-0.5 rounded-md px-1 text-xs font-medium text-fg transition-colors hover:bg-surface-hover"
        >
          {name(value)}
          <Icon as={IconChevronDown} size={12} className="opacity-60" />
        </button>
      )}
    >
      {(close) => (
        <div className="w-40">
          {operators.map((op) => (
            <PopoverItem
              key={op}
              onClick={() => {
                onChange(op);
                close();
              }}
            >
              {name(op)}
            </PopoverItem>
          ))}
        </div>
      )}
    </Popover>
  );
}

/** Короткая подпись значения на чипе — чтобы не открывать его ради проверки. */
function summary(
  filter: Filter,
  field: Field,
  language: string,
  t: (key: TranslationKey) => string,
): string {
  const values = filter.values.filter(Boolean);

  if (filter.op === "any") {
    const [first, ...rest] = values.map((item) => {
      const option = field.options.get(item);
      return option ? localized(option.labels, language, option.label || option.value) : item;
    });
    return rest.length ? `${first} +${rest.length}` : (first ?? "");
  }

  if (filter.op === "equals") return t(values[0] === "true" ? "action.yes" : "action.no");
  if (filter.op === "between") return values.join(" — ");

  return values[0] ?? "";
}

function Values({
  field,
  language,
  filter,
  onChange,
}: {
  field: Field;
  language: string;
  filter: Filter;
  onChange: (filter: Filter) => void;
}) {
  const setValues = (values: string[]) => onChange({ ...filter, values });

  switch (kindOfOperator(filter.op)) {
    case "set":
      return <SetInput field={field} language={language} values={filter.values} onChange={setValues} />;
    case "boolean":
      return <BooleanInput field={field} values={filter.values} onChange={setValues} />;
    case "range":
      return <RangeInput op={filter.op} values={filter.values} onChange={setValues} />;
    default:
      return <TextInput values={filter.values} onChange={setValues} />;
  }
}

function SetInput({
  field,
  language,
  values,
  onChange,
}: {
  field: Field;
  language: string;
  values: string[];
  onChange: (values: string[]) => void;
}) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");

  const options = [...field.options.values()].filter((option) =>
    localized(option.labels, language, option.label || option.value)
      .toLowerCase()
      .includes(query.trim().toLowerCase()),
  );

  return (
    <div className="flex flex-col gap-1 p-1">
      <input
        autoFocus
        value={query}
        onChange={(event) => setQuery(event.target.value)}
        placeholder={t("table.filterChoose")}
        className="h-7 w-full rounded-md border border-border-strong bg-surface px-2 text-sm text-fg"
      />

      <div className="max-h-64 overflow-y-auto">
        {options.map((option) => (
          <label
            key={option.value}
            className="flex h-8 cursor-pointer items-center gap-2 rounded-md px-1 text-sm transition-colors hover:bg-surface-hover"
          >
            <Checkbox
              checked={values.includes(option.value)}
              onChange={() =>
                onChange(
                  values.includes(option.value)
                    ? values.filter((item) => item !== option.value)
                    : [...values, option.value],
                )
              }
            />
            <span className="truncate">
              {localized(option.labels, language, option.label || option.value)}
            </span>
          </label>
        ))}

        {!options.length && (
          <p className="px-1 py-2 text-xs text-fg-subtle">{t("table.noOptions")}</p>
        )}
      </div>
    </div>
  );
}

/**
 * Текст применяется по вводу с задержкой, а не по кнопке: правило то же,
 * что у общего поиска, — иначе каждая буква уходит запросом.
 */
function TextInput({
  values,
  onChange,
}: {
  values: string[];
  onChange: (values: string[]) => void;
}) {
  const { t } = useTranslation();
  const [text, setText] = useState(values[0] ?? "");

  return (
    <form
      className="p-1"
      onSubmit={(event) => {
        event.preventDefault();
        onChange(text ? [text] : []);
      }}
    >
      <input
        autoFocus
        value={text}
        onChange={(event) => setText(event.target.value)}
        onBlur={() => onChange(text ? [text] : [])}
        placeholder={t("table.filterValue")}
        className="h-7 w-full rounded-md border border-border-strong bg-surface px-2 text-sm text-fg"
      />
    </form>
  );
}

function BooleanInput({
  field,
  values,
  onChange,
}: {
  field: Field;
  values: string[];
  onChange: (values: string[]) => void;
}) {
  const { t } = useTranslation();

  // Подписи «да» и «нет» задаются в настройках поля.
  const labels: Record<string, string> = {
    true: asText(field.attributes["text_true"]) || t("action.yes"),
    false: asText(field.attributes["text_false"]) || t("action.no"),
  };

  return (
    <div className="flex flex-col p-1">
      {["true", "false"].map((option) => (
        <button
          key={option}
          type="button"
          onClick={() => onChange([option])}
          className={`flex h-8 items-center rounded-md px-1 text-left text-sm transition-colors hover:bg-surface-hover ${
            values[0] === option ? "text-accent-text" : "text-fg"
          }`}
        >
          {labels[option]}
        </button>
      ))}
    </div>
  );
}

/**
 * Даты — нативные <input type="date">.
 *
 * Родной элемент даёт календарь, локальный формат и ввод с клавиатуры
 * бесплатно; библиотека выбора дат весит больше всей этой панели.
 */
function RangeInput({
  op,
  values,
  onChange,
}: {
  op: FilterOperator;
  values: string[];
  onChange: (values: string[]) => void;
}) {
  const { t } = useTranslation();
  const input = "h-7 w-full rounded-md border border-border-strong bg-surface px-2 text-sm text-fg";

  const at = (index: number) => values[index] ?? "";
  const put = (index: number, value: string) => {
    const next = [at(0), at(1)];
    next[index] = value;
    // Хвостовые пустые убираем, иначе «заполнен» станет правдой раньше времени.
    onChange(next[1] ? next : next[0] ? [next[0]] : []);
  };

  if (op !== "between") {
    return (
      <div className="p-1">
        <input
          type="date"
          value={at(0)}
          onChange={(event) => put(0, event.target.value)}
          className={input}
        />
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-1 p-1">
      <label className="flex items-center gap-2 text-xs text-fg-muted">
        <span className="w-6 shrink-0">{t("table.filterFrom")}</span>
        <input
          type="date"
          value={at(0)}
          onChange={(event) => put(0, event.target.value)}
          className={input}
        />
      </label>

      <label className="flex items-center gap-2 text-xs text-fg-muted">
        <span className="w-6 shrink-0">{t("table.filterTo")}</span>
        <input
          type="date"
          value={at(1)}
          onChange={(event) => put(1, event.target.value)}
          className={input}
        />
      </label>
    </div>
  );
}

function asText(value: unknown): string {
  return typeof value === "string" ? value : "";
}
