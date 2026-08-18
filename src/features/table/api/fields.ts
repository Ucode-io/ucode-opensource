import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { CHIP_HEX } from "@/shared/ui/chip";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { reportError, toast } from "@/shared/lib/toast";
import type { Field } from "../model/types";
import {
  MULTILANGUAGE_TYPES,
  optionsShape,
  STATUS_GROUPS,
  type DraftOption,
  type FieldDraft,
} from "../model/field-draft";
import { slugify } from "@/shared/lib/slug";

/**
 * Создание поля.
 *
 * Ручка одна — POST /v2/fields/{table}, — но тело у неё историческое,
 * и три места в нём нельзя понять из имени:
 *
 *   id            придумывает КЛИЕНТ. Бэкенд вставляет его в колонку
 *                 как есть (field.go: req.GetId()), пустой не заменяет.
 *   table_id      слаг ТАБЛИЦЫ, а не id и не поля: ручка принимает и
 *                 uuid, и слаг — там явная ветка `uuid.Parse`. Ошибиться
 *                 легко, а ответ на чужой слаг — 500 «not found».
 *   required      в INSERT захардкожен в false. Галочку «обязательное»
 *                 здесь не показываем: она бы ничего не делала.
 *
 * View трогать не нужно: создание поля само дописывает его во все view
 * таблицы (`UPDATE "view" SET columns = array_append(...)`). Поэтому
 * после успеха инвалидируется и схема, и список view — иначе колонка
 * появится только после перезагрузки страницы.
 */
export function useCreateField(tableSlug: string | undefined) {
  const queryClient = useQueryClient();
  const slug = tableSlug ?? "";

  return useMutation({
    mutationFn: ({ draft, language }: { draft: FieldDraft; language: string }) =>
      api.post<unknown>(
        `/v2/fields/${slug}`,
        toCreateBody(draft, { tableSlug: slug, language, id: crypto.randomUUID() }),
      ),
    onError: (error) => reportError(error, "common.createFailed"),
    onSuccess: (_data, { draft }) => {
      // Колонка встаёт последней и часто оказывается за краем экрана:
      // без уведомления создание выглядит как «ничего не произошло».
      toast.success(i18n.t("fieldForm.created", { label: draft.label.trim() }));
      invalidateSchema(queryClient);
    },
  });
}

/**
 * Правка поля.
 *
 * Тело — исходный ответ сервера с заменёнными полями, а не собранный
 * заново объект: ручка перезаписывает запись целиком, и всё, чего
 * в теле не окажется, обнулится (см. Field.raw).
 *
 * Слаг не меняем никогда. Бэкенд его умеет — делает RENAME COLUMN, —
 * но у него же есть ветка «сменился тип»: она сначала дропает колонку
 * по СТАРОМУ слагу, а потом переименовывает её по новому. Два изменения
 * сразу ломают запрос, и разбираться в этом ради переименования
 * колонки, которую видит только база, незачем.
 */
export function useUpdateField(tableSlug: string | undefined) {
  const queryClient = useQueryClient();
  const slug = tableSlug ?? "";

  return useMutation({
    mutationFn: ({ field, draft, language }: { field: Field; draft: FieldDraft; language: string }) =>
      api.put<unknown>(`/v2/fields/${slug}`, toUpdateBody(field, draft, language)),

    onError: (error) => reportError(error, "common.saveFailed"),
    onSuccess: () => invalidateSchema(queryClient),
  });
}

/**
 * Удаление поля. Вместе с ним уходит и колонка в базе — со всеми
 * значениями во всех строках. Спрашивать подтверждение обязан
 * вызывающий: отсюда этого уже не видно.
 */
export function useDeleteField(tableSlug: string | undefined) {
  const queryClient = useQueryClient();
  const slug = tableSlug ?? "";

  return useMutation({
    mutationFn: (field: Field) => api.delete<unknown>(`/v2/fields/${slug}/${field.id}`),

    onError: (error) => reportError(error, "common.deleteFailed"),
    onSuccess: (_data, field) => {
      toast.success(i18n.t("fieldForm.deleted", { label: field.label }));
      invalidateSchema(queryClient);
    },
  });
}

/**
 * Какие поля участвуют в общем поиске.
 *
 * Отдельная ручка и отдельный флаг в базе: поиск на бэкенде — это
 * `a.<колонка> ~* $1` по отмеченным колонкам, и без единой отметки
 * условие не собирается вовсе, а поиск тихо возвращает всю таблицу.
 * PUT принимает список пар {id, is_search} и проходит по ним UPDATE'ом,
 * поэтому шлём только то, что переключили.
 */
export function useUpdateSearchFields(tableSlug: string | undefined) {
  const queryClient = useQueryClient();
  const slug = tableSlug ?? "";

  return useMutation({
    mutationFn: (fields: { id: string; searchable: boolean }[]) =>
      api.put<unknown>(`/v2/fields/${slug}/update-search`, {
        fields: fields.map((field) => ({ id: field.id, is_search: field.searchable })),
      }),

    onError: (error) => reportError(error, "common.saveFailed"),
    // Только флаги: инвалидация всей схемы тянула бы за собой поля,
    // связи и запрос на КАЖДУЮ связь — полтора десятка запросов
    // на один щелчок по галочке.
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: keys.tables.searchFields(slug) }),
  });
}

/**
 * Схема поменялась — перезапрашиваем и её, и view: создание и удаление
 * поля правят список колонок во всех view таблицы (`UPDATE "view" SET
 * columns = array_append(...)`). Иначе колонка появится или исчезнет
 * только после перезагрузки страницы.
 */
export function invalidateSchema(queryClient: ReturnType<typeof useQueryClient>) {
  void queryClient.invalidateQueries({ queryKey: keys.tables.all });
  void queryClient.invalidateQueries({ queryKey: keys.views.all });
}

/**
 * Черновик поверх исходного ответа. Меняются ровно четыре вещи:
 * подпись, тип, подпись на языке данных и список вариантов.
 *
 * Варианты прежнего типа стираются, если новый тип их не знает: иначе
 * у обычной строки остаётся мёртвый список вариантов в настройках,
 * и следующий, кто сменит тип обратно, получит его из ниоткуда.
 */
export function toUpdateBody(
  field: Field,
  draft: FieldDraft,
  language: string,
): Record<string, unknown> {
  const label = draft.label.trim();
  const options = toOptionAttributes(draft, language);

  return {
    ...field.raw,
    id: field.id,
    slug: field.slug,
    label,
    type: draft.type,
    required: draft.required,
    unique: draft.unique,
    ...toColumnSettings(draft),
    attributes: {
      ...withoutOptions(field.attributes),
      ...labelAttributes(draft, language, label),
      ...toSettingsAttributes(draft),
      ...options,
    },
  };
}

/**
 * Подписи по языкам: `label_<код>` за каждый заполненный язык.
 *
 * Активный язык берётся из `label` — это то, что человек видит в поле
 * ввода, и оно же уходит в колонку `label`. Пустые языки не пишутся:
 * пустой `label_uz` перебил бы подпись в старой админке, которая
 * читает его первым.
 */
function labelAttributes(
  draft: FieldDraft,
  language: string,
  label: string,
): Record<string, string> {
  const attributes: Record<string, string> = {};

  for (const [code, text] of Object.entries(draft.labels)) {
    const value = text.trim();
    if (value) attributes[`label_${code}`] = value;
  }

  if (label) attributes[`label_${language}`] = label;

  return attributes;
}

/**
 * Настройки, которые живут КОЛОНКАМИ таблицы field, а не в attributes:
 * автозаполнение и мультиязычность. Одинаковы в обоих телах, поэтому
 * собраны один раз.
 *
 * В attributes те же имена тоже встречаются — бэкенд копирует их туда,
 * отдавая layout, — но записать настройку через attributes нельзя:
 * UPDATE читает только колонки (field.go). Отправишь в attributes —
 * значение сохранится и не заработает.
 *
 * Мультиязычность отправляется только у типов, которые её умеют:
 * у остальных флаг остался бы висеть в базе после смены типа, а поля
 * на второй язык всё равно никто не заведёт.
 */
function toColumnSettings(draft: FieldDraft): Record<string, unknown> {
  return {
    autofill_table: draft.autofillTable,
    autofill_field: draft.autofillField,
    automatic: draft.automatic,
    enable_multilanguage: MULTILANGUAGE_TYPES.has(draft.type) && draft.multilanguage,
  };
}

/**
 * Настройки, которые живут в attributes, а не колонками.
 *
 * `disabled` читает и наша схема (Field.editable), и старая админка —
 * это одно и то же «править нельзя». Число цифр отправляется числом:
 * бэкенд приводит его через cast.ToInt, но строку «08» приведёт к 8,
 * а пустую — к нулю, что значит «девять цифр».
 */
function toSettingsAttributes(draft: FieldDraft): Record<string, unknown> {
  const digits = Number(draft.digits.trim());

  return {
    disabled: draft.readonly,
    /*
     * Проверка ввода. Оба ключа отправляются всегда, в том числе
     * пустыми: иначе стёртое выражение осталось бы в attributes
     * (тело собирается поверх прежнего, см. toUpdateBody) и продолжало
     * бы отклонять правки, которые человек только что разрешил.
     */
    validation: draft.validation.trim(),
    validation_message: draft.validationMessage.trim(),
    ...toFormulaAttributes(draft),
    /*
     * Кнопка. Ключи чужого типа не отправляются вовсе — по той же
     * причине, что у формул: у поля, которое перестало быть кнопкой,
     * в attributes осталась бы мёртвая функция, и следующий читатель
     * принял бы её за настройку.
     */
    ...(draft.type === "BUTTON"
      ? { icon: draft.icon.trim(), function: draft.functionId }
      : {}),
    ...(draft.type === "INCREMENT_ID" && Number.isInteger(digits) && digits > 0 && digits < 10
      ? { digit_number: digits }
      : {}),
  };
}

/**
 * Настройки формул. Два разных поля под одним словом:
 *
 *   FORMULA_FRONTEND  выражение считает браузер (attributes.formula)
 *   FORMULA           агрегат по связанной таблице, считает бэкенд
 *
 * Ключи агрегата исторические и в attributes лежат россыпью — их
 * разбор описан в model/field-draft. `number_of_rounds` уходит числом:
 * бэкенд приводит его через cast.ToInt, но строку «» приведёт к нулю
 * молча, а нам нужно отличать «не задано» от нуля.
 *
 * Ключи чужого типа не отправляются вовсе: у поля, которое перестало
 * быть формулой, в attributes остался бы мёртвый агрегат, и следующий
 * читатель принял бы его за настройку.
 */
function toFormulaAttributes(draft: FieldDraft): Record<string, unknown> {
  if (draft.type === "FORMULA_FRONTEND") return { formula: draft.formula.trim() };
  if (draft.type !== "FORMULA") return {};

  const { type, tableFrom, field, rounds, filters } = draft.aggregate;
  const digits = Number(rounds.trim());

  return {
    type,
    table_from: tableFrom,
    sum_field: field,
    ...(rounds.trim() && Number.isFinite(digits) ? { number_of_rounds: digits } : {}),
    // Условие без поля — это пустая строка, которую человек добавил
    // и не заполнил: бэкенд по ней отберёт ничего.
    formula_filters: filters.filter((filter) => filter.key),
  };
}

/** Ключи, в которых живут варианты выбора обоих типов. */
const OPTION_KEYS = ["options", "has_color", ...STATUS_GROUPS];

function withoutOptions(attributes: Record<string, unknown>): Record<string, unknown> {
  const rest = { ...attributes };
  for (const key of OPTION_KEYS) delete rest[key];
  return rest;
}

/**
 * Черновик → тело запроса. Отдельная функция, потому что проверяется
 * тестом: ошибка здесь тихая — поле создастся, но без вариантов или
 * без подписи, и это выяснится через день.
 */
export function toCreateBody(
  draft: FieldDraft,
  { tableSlug, language, id }: { tableSlug: string; language: string; id: string },
): Record<string, unknown> {
  const label = draft.label.trim();

  return {
    id,
    slug: draft.slug.trim() || slugify(label),
    label,
    type: draft.type,
    /*
     * Именно ТАБЛИЦА, а не поле. Имя ключа врёт дважды: это не id
     * и не поля — бэкенд ищет `WHERE slug = $1`, если значение не uuid.
     * Слаг таблицы у нас есть всегда, её id — нет.
     */
    table_id: tableSlug,
    index: "",
    is_visible: true,
    show_label: true,
    // Обязательность бэкенд при вставке игнорирует (пишет false), поэтому
    // её и не спрашиваем в форме нового поля — см. FieldDraft.required.
    required: false,
    unique: draft.unique,
    /*
     * enable_multilanguage при СОЗДАНИИ работает иначе, чем при правке:
     * шлюз по нему заводит по полю на каждый язык проекта — `title_en`,
     * `title_cyr` (field.go: SetTitlePrefix). Сама колонка при этом
     * останется false, потому что INSERT в object_builder её
     * не перечисляет: в форме поле снова покажется выключенным, пока
     * его не сохранят ещё раз.
     */
    ...toColumnSettings(draft),
    attributes: {
      // Подписи на языках ДАННЫХ: их читает и ячейка, и старый фронт.
      ...labelAttributes(draft, language, label),
      ...toSettingsAttributes(draft),
      ...toOptionAttributes(draft, language),
    },
  };
}

/**
 * Варианты выбора. Форматы у двух типов разные, и это не вкусовщина:
 * у STATUS варианты разложены по стадиям, у MULTISELECT — одним списком
 * (см. CONTEXT, FieldOption).
 *
 * Ключ, по которому значение потом ищется в строке, у них тоже разный:
 * MULTISELECT кладёт в строку `slug`, STATUS — `value`. Поэтому
 * человеческая подпись у первого лежит в `value`, а у второго только
 * в `label_<язык>`.
 */
function toOptionAttributes(draft: FieldDraft, language: string): Record<string, unknown> {
  const shape = optionsShape(draft.type);
  if (!shape) return {};

  if (shape === "flat") {
    return {
      has_color: true,
      options: draft.options.filter(named).map((option) => toFlatOption(option, language)),
    };
  }

  const attributes: Record<string, unknown> = { has_color: true, options: [] };

  for (const group of STATUS_GROUPS) {
    attributes[group] = {
      options: draft.groups[group].filter(named).map((option) => toStatusOption(option, language)),
    };
  }

  return attributes;
}

function toFlatOption(option: DraftOption, language: string): Record<string, unknown> {
  const label = option.label.trim();

  return {
    /*
     * slug — это то, что ляжет в строку. Отдельный от подписи токен,
     * а не сама подпись: иначе переименование варианта осиротит все
     * значения, уже сохранённые под старым текстом. Старый ucode писал
     * сюда подпись целиком.
     *
     * У существующего варианта он берётся как есть и не пересчитывается
     * по той же причине: строки уже проставлены старым значением.
     */
    slug: option.value ?? (slugify(label) || label),
    value: label,
    label,
    [`label_${language}`]: label,
    color: CHIP_HEX[option.color],
  };
}

function toStatusOption(option: DraftOption, language: string): Record<string, unknown> {
  const label = option.label.trim();

  return {
    // Существующее значение переживает переименование: под ним уже
    // лежат строки (см. DraftOption.value).
    value: option.value ?? (slugify(label) || label),
    // label у STATUS не читает никто, кроме нас: держим его как запасной
    // вариант подписи, если в проекте появится ещё один язык данных.
    label,
    [`label_${language}`]: label,
    color: CHIP_HEX[option.color],
  };
}

function named(option: DraftOption): boolean {
  return option.label.trim().length > 0;
}
