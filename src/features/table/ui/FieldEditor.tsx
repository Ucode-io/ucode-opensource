import { useEffect, useMemo, useRef, useState } from "react";
import {
  IconCheck,
  IconChevronLeft,
  IconChevronRight,
  IconPlus,
  IconSearch,
  IconTrash,
  type Icon as TablerIcon,
} from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { Anchored } from "@/shared/ui/anchored";
import { Checkbox } from "@/shared/ui/checkbox";
import { CHIP_COLORS, Chip } from "@/shared/ui/chip";
import { Icon } from "@/shared/ui/icon";
import { LanguageInput } from "@/shared/ui/language-input";
import { SelectMenu } from "@/shared/ui/select-menu";
import type { TranslationKey } from "@/shared/lib/i18n";
import { isValidSlug, slugify } from "@/shared/lib/slug";
import {
  DEFAULT_LENGTH,
  EMPTY_DRAFT,
  FIELD_TYPE_GROUPS,
  MULTILANGUAGE_TYPES,
  STATUS_GROUPS,
  fieldTypeLabel,
  hasDefaultValue,
  hasLength,
  hasPrefix,
  hasScannerSettings,
  isValidPattern,
  newDraft,
  optionsShape,
  toBlocks,
  toDraft,
  withType,
  type DraftOption,
  type FieldDraft,
  RELATION_TYPE,
  type FieldTypeGroup,
  type StatusGroup,
} from "../model/field-draft";
import {
  EMPTY_RELATION_DRAFT,
  isRelationReady,
  toAutoFilters,
  type AutoFilterPair,
  type RelationDraft,
} from "../model/relation-draft";
import { localized, type Field, type Relation } from "../model/types";
import { useTableFields, useTables } from "../api/tables";
import { AutofillSettings } from "./AutofillSettings";
import { ButtonSettings } from "./ButtonSettings";
import { FormulaSettings } from "./FormulaSettings";
import { TypeSettings } from "./TypeSettings";

/**
 * Поле: заводится и правится одной панелью, привязанной к тому месту,
 * откуда её открыли, — к заголовку колонки, к кнопке «+» в шапке или
 * к раскрытой ячейке.
 *
 * Панель, а не модалка, потому что правка поля — это правка того, что
 * человек прямо сейчас видит: варианты статуса нужно сверять с колонкой
 * под ними, а не с затемнённым экраном. По той же причине здесь нет
 * кнопки «Сохранить»: правки копятся в черновике и уезжают ОДНИМ PUT
 * при закрытии. Запрос на каждую букву в названии варианта — это
 * полсотни запросов на одно слово.
 *
 * Создание устроено наоборот: тип выбирают кликом, и этот клик и есть
 * подтверждение. Имя необязательно — поле назовётся своим типом.
 */
export function FieldEditor({
  field,
  fields,
  relations,
  language,
  languages,
  anchor,
  icon,
  onSubmit,
  onSubmitRelation,
  onEditRelation,
  onDelete,
  onClose,
}: {
  /** Правим существующее поле. Нет — заводим новое. */
  field?: Field | undefined;
  /** Все поля таблицы: имя и слаг нового не должны повторить чужие. */
  fields: Field[];
  /** Связи таблицы: по ним выбирается таблица агрегата у FORMULA. */
  relations: Relation[];
  language: string;
  /**
   * Языки ДАННЫХ проекта: на них пишется подпись поля, и по ним же
   * шлюз заводит языковые колонки мультиязычного поля.
   */
  languages: { code: string; nativeName: string }[];
  anchor: DOMRect;
  /** Иконка типа. Параметром, а не импортом — см. TypeList. */
  icon: (type: string) => TablerIcon;
  onSubmit: (draft: FieldDraft) => void;
  /**
   * Завести связь, а не поле. Связь — отдельная сущность с отдельной
   * ручкой (POST /v2/relations), и колонку-ссылку бэкенд добавляет сам.
   *
   * Не задан — тип «Связь» в списке не показывается: предлагать выбор,
   * который ничем не заканчивается, хуже, чем не предлагать.
   */
  onSubmitRelation?: ((draft: RelationDraft) => void) | undefined;
  /**
   * Правка существующей связи: подпись и поля показа. Целевая таблица
   * не меняется — это была бы другая колонка в базе.
   */
  onEditRelation?: ((relation: Relation, draft: RelationDraft) => void) | undefined;
  onDelete: (field: Field) => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const editing = Boolean(field);

  const initial = useMemo(
    () => (field ? toDraft(field, language) : EMPTY_DRAFT),
    [field, language],
  );
  const [draft, setDraft] = useState<FieldDraft>(initial);
  /** Открыт список типов. У нового поля с него всё и начинается. */
  const [choosingType, setChoosingType] = useState(!editing);
  /** Открыт вариант выбора: его имя, цвет и удаление — отдельным экраном. */
  const [openOption, setOpenOption] = useState<OptionRef | null>(null);
  /** Слаг, тронутый руками, больше не переписывается из названия. */
  const [slugTouched, setSlugTouched] = useState(false);

  const patch = (next: Partial<FieldDraft>) => setDraft((value) => ({ ...value, ...next }));

  const slug = draft.slug.trim();
  /*
   * Только при создании: у существующего поля слаг совпадает сам с собой
   * и «занят» был бы всегда. Правится он тоже только здесь — переименовать
   * колонку после создания нельзя (см. api/fields).
   */
  const slugError =
    !editing && Boolean(slug && (!isValidSlug(slug) || fields.some((item) => item.slug === slug)));

  /**
   * Закрытие — это и есть «сохранить»: уезжает ровно одно изменение.
   *
   * У связи не так: её правки уходят кнопкой, а закрытие ничего
   * не сохраняет. Иначе PUT /v2/fields уехал бы по колонке-ссылке,
   * которой он не управляет.
   */
  const close = () => {
    if (!editingRelation && field && JSON.stringify(draft) !== JSON.stringify(initial)) {
      onSubmit(draft);
    }
    onClose();
  };

  const create = (type: string) => {
    if (slugError) return;

    /*
     * Мультиязычность выбирается ДО типа, потому что она работает только
     * при создании: шлюз по ней заводит по полю на каждый язык проекта
     * (handlers/v2/field.go:84 — SetTitlePrefix). У уже созданного поля
     * тот же флаг только меняет колонку в базе, а языковых полей не
     * появляется — включать его там нечем.
     */
    const ready = newDraft(type, draft.label, fields);
    onSubmit({
      ...ready,
      labels: draft.labels,
      /*
       * Флаг едет только с текстовым типом: шлюз разводит поле
       * по языковым колонкам ровно для SINGLE_LINE и MULTI_LINE
       * (handlers/v2/field.go:84), а у остальных молча заводит одну
       * колонку. Тип выбирается последним, поэтому проверка здесь,
       * а не у самого переключателя.
       */
      multilanguage: draft.multilanguage && MULTILANGUAGE_TYPES.has(type),
      ...(slug && slugTouched ? { slug } : {}),
    });
    onClose();
  };

  /*
   * Связь — не тип колонки, а отдельная сущность: у неё своя ручка, и
   * слаг колонки-ссылки задаёт целевая таблица, а не человек. Поэтому
   * выбор «Связь» не создаёт ничего сразу, как остальные типы, а
   * открывает форму: без целевой таблицы создавать нечего.
   */
  /*
   * Открыли поле-связь — это правка связи, а не поля. У связи своя
   * ручка и свои настройки, а PUT /v2/fields колонку-ссылку бы только
   * испортил: он не знает ни целевой таблицы, ни полей показа.
   */
  const editingRelation = field?.relationId
    ? relations.find((item) => item.id === field.relationId)
    : undefined;

  const [relation, setRelation] = useState<RelationDraft | null>(() =>
    editingRelation
      ? {
          toSlug: editingRelation.toSlug,
          autoFilters: toAutoFilters(editingRelation.raw),
          label: localized(field?.labels ?? {}, language, field?.label ?? ""),
          viewFieldIds: editingRelation.viewFieldIds,
          selfDefault: editingRelation.selfDefault,
        }
      : null,
  );

  const pickType = (type: string) => {
    if (type === RELATION_TYPE) {
      setRelation({ ...EMPTY_RELATION_DRAFT, label: draft.label });
      setChoosingType(false);
      return;
    }

    if (!editing) return create(type);

    setDraft((value) => withType(value, type));
    setChoosingType(false);
  };

  /*
   * Выход из списка типов, ничего не выбрав.
   *
   * У существующего поля список — это ШАГ, на который зашли из настроек,
   * и единственным выходом из него не может быть смена типа: она стирает
   * колонку в базе. Раньше выхода не было вовсе — панель приходилось
   * закрывать и открывать заново, теряя вместе с ней все правки.
   *
   * У нового поля возвращаться некуда: список и есть первый экран.
   */
  const back = editing ? () => setChoosingType(false) : null;
  const closeOption = () => setOpenOption(null);

  /*
   * Смена типа у существующего поля — это DROP COLUMN плюс ADD COLUMN
   * на стороне базы (field.go). Значения во всех строках пропадают,
   * и сказать об этом надо ДО закрытия панели, а не после.
   */
  const retyping = Boolean(field && draft.type !== initial.type);
  const shape = optionsShape(draft.type);

  const setGroup = (group: StatusGroup, options: DraftOption[]) =>
    patch({ groups: { ...draft.groups, [group]: options } });

  /*
   * Варианты у двух типов лежат по-разному: у STATUS — тремя списками
   * по стадиям, у MULTISELECT — одним (см. CONTEXT, FieldOption). Ссылка
   * на вариант поэтому состоит из стадии и номера, а не из одного номера.
   */
  const optionsIn = (group: StatusGroup | null) => (group ? draft.groups[group] : draft.options);
  const putOptions = (group: StatusGroup | null, options: DraftOption[]) =>
    group ? setGroup(group, options) : patch({ options });

  /** Новый вариант сразу открывается: имя ему всё равно набирать. */
  const addOption = (group: StatusGroup | null) => {
    const list = optionsIn(group);

    // Цвет по очереди, а не случайный: два соседних варианта не окажутся
    // одного оттенка.
    putOptions(group, [
      ...list,
      { label: "", color: CHIP_COLORS[list.length % CHIP_COLORS.length]! },
    ]);
    setOpenOption({ group, index: list.length });
  };

  const patchOption = (next: Partial<DraftOption>) => {
    if (!openOption) return;

    putOptions(
      openOption.group,
      optionsIn(openOption.group).map((option, index) =>
        index === openOption.index ? { ...option, ...next } : option,
      ),
    );
  };

  const deleteOption = () => {
    if (!openOption) return;

    putOptions(
      openOption.group,
      optionsIn(openOption.group).filter((_, index) => index !== openOption.index),
    );
    closeOption();
  };

  /** Вариант, чей экран открыт. Пусто — значит его только что удалили. */
  const option = openOption ? optionsIn(openOption.group)[openOption.index] : undefined;

  return (
    // Escape на вложенном экране — это «назад», а не «закрыть»: закрытие
    // отсюда выбрасывало бы и всё, что человек уже наменял в панели.
    <Anchored
      anchor={anchor}
      onClose={close}
      onCancel={option ? closeOption : choosingType && back ? back : onClose}
    >
      {/* Список типов шире остальной панели: в 288 пикселей две колонки
          влезают, но подпись вроде «Date and time (no time zone)» в них
          обрезается до «Date and t…» — выбирать не из чего. */}
      <div
        className={`flex max-h-[70vh] flex-col overflow-hidden rounded-lg border border-border bg-surface p-1 shadow-popover ${
          choosingType && !relation ? "w-[26rem]" : "w-72"
        }`}
      >
        {relation ? (
          <RelationForm
            draft={relation}
            target={editingRelation?.toSlug ?? ""}
            ourFields={fields}
            language={language}
            onChange={setRelation}
            onBack={editingRelation ? onClose : () => {
              setRelation(null);
              setChoosingType(true);
            }}
            onSubmit={() => {
              if (editingRelation) onEditRelation?.(editingRelation, relation);
              else onSubmitRelation?.(relation);
              onClose();
            }}
          />
        ) : option ? (
          <OptionScreen
            option={option}
            onChange={patchOption}
            onDelete={deleteOption}
            onBack={closeOption}
          />
        ) : (
          <>
            <div className="flex items-center gap-1.5 p-1">
              <span className="grid size-7 shrink-0 place-items-center rounded-md border border-border text-fg-muted">
                <Icon as={icon(draft.type)} size={14} />
              </span>

              {/*
                Подпись на каждом языке ДАННЫХ — одним полем с
                переключателем внутри, как в старой админке
                (TextFieldWithMultiLanguage в FieldCreateModal). Пока
                правился только активный язык, поле, названное по-русски,
                оставалось `title` в узбекском интерфейсе.

                Слаг подсказывается по подписи на АКТИВНОМ языке: имя
                колонки в базе одно, и собирать его из четырёх языков
                нечем.
              */}
              <span className="flex min-w-0 flex-1 items-center">
                <LanguageInput
                  autoFocus
                  languages={languages}
                  values={{ ...draft.labels, [language]: draft.label }}
                  placeholder={t("fieldForm.namePlaceholder")}
                  label={t("fieldForm.label")}
                  onChange={(code, label) =>
                    patch({
                      labels: { ...draft.labels, [code]: label },
                      ...(code === language
                        ? {
                            label,
                            ...(slugTouched || editing ? {} : { slug: slugify(label) }),
                          }
                        : {}),
                    })
                  }
                  onEnter={() => (editing ? close() : create(draft.type))}
                />
              </span>
            </div>

            {/*
              Имя колонки в базе. Показано и правится только при создании:
              человек должен видеть, как будет называться колонка, а после
              создания переименовать её нельзя (см. api/fields).
            */}
            {!editing && (
              <input
                value={draft.slug}
                aria-label={t("fieldForm.slug")}
                placeholder={t("fieldForm.slug")}
                onChange={(event) => {
                  setSlugTouched(true);
                  patch({ slug: event.target.value });
                }}
                className={`mx-1 h-6 rounded-md bg-transparent px-2 font-mono text-2xs outline-none transition-colors hover:bg-surface-hover focus:bg-surface-hover ${
                  slugError ? "text-danger" : "mb-1 text-fg-subtle"
                }`}
              />
            )}

            {/*
              Мультиязычность спрашивается здесь, а не в настройках поля:
              она действует только в момент создания — шлюз заводит по
              колонке на каждый язык проекта и суффиксует слаг
              (`title_en`, `title_cyr`). Позже включить её нечем: у поля
              меняется флаг, а языковых колонок не появляется.

              Работает она только у текстовых типов, и подпись об этом
              говорит: тип здесь выбирают последним щелчком, поэтому
              спрятать переключатель по типу нельзя — с чужим типом
              флаг просто не уедет (см. create).

              У проекта с одним языком спрашивать нечего.
            */}
            {!editing && languages.length > 1 && (
              <label className="mx-1 mb-1 flex cursor-pointer items-start gap-2 rounded-md px-2 py-1 transition-colors hover:bg-surface-hover">
                <span className="pt-0.5">
                  <Checkbox
                    checked={draft.multilanguage}
                    onChange={(event) => patch({ multilanguage: event.target.checked })}
                  />
                </span>
                <span className="flex min-w-0 flex-col">
                  <span className="text-sm text-fg">{t("fieldForm.multilanguage")}</span>
                  <span className="text-2xs text-fg-subtle">
                    {t("fieldForm.multilanguageHint", {
                      languages: languages.map((item) => item.code).join(", "),
                    })}
                  </span>
                </span>
              </label>
            )}

            {/* Пока слаг занят или неправилен, клик по типу поле не создаёт
                (create молча выходит) — без подписи это выглядит так, будто
                панель сломалась. */}
            {slugError && (
              <p className="mx-1 mb-1 px-2 text-2xs text-danger">{t("fieldForm.slugInvalid")}</p>
            )}

            {choosingType ? (
              <TypeList
                value={draft.type}
                icon={icon}
                onPick={pickType}
                onBack={back}
                // Связь заводится только заново: у существующего поля
                // смена типа — это DROP COLUMN, а связь колонкой не является.
                withRelation={!editing && Boolean(onSubmitRelation)}
                /* У нового поля фокус остаётся в названии: его набирают
                   первым. У существующего — в поиске: список открыли
                   намеренно, и первое же нажатие клавиши должно фильтровать,
                   а не уходить в никуда. */
                autoFocus={editing}
              />
            ) : (
              <div className="min-h-0 flex-1 overflow-y-auto">
                <Row
                  label={t("fieldForm.type")}
                  value={fieldTypeLabel(draft.type)}
                  onClick={() => setChoosingType(true)}
                />

                {retyping && (
                  <p className="m-1 rounded-md border border-danger bg-danger-subtle p-2 text-2xs text-danger">
                    {t("fieldForm.retypeWarning")}
                  </p>
                )}

                {shape === "groups" &&
                  STATUS_GROUPS.map((group) => (
                    <section key={group}>
                      <header className="flex h-7 items-center gap-1 px-1.5">
                        <span className="flex-1 text-2xs text-fg-muted">{t(`status.${group}`)}</span>
                        <AddOption onClick={() => addOption(group)} />
                      </header>

                      <OptionList
                        options={draft.groups[group]}
                        onOpen={(index) => setOpenOption({ group, index })}
                      />
                    </section>
                  ))}

                {shape === "flat" && (
                  <section>
                    <header className="flex h-7 items-center gap-1 px-1.5">
                      <span className="flex-1 text-2xs text-fg-muted">{t("fieldForm.options")}</span>
                      <AddOption onClick={() => addOption(null)} />
                    </header>

                    <OptionList
                      options={draft.options}
                      onOpen={(index) => setOpenOption({ group: null, index })}
                    />
                  </section>
                )}

                {/*
                  Настройки, которые бэкенд действительно применяет.
                  Обязательность — только у существующего поля: при создании
                  она в базу не доезжает (см. api/fields).
                */}
                <div className="my-1 h-px bg-border" />

                {editing && (
                  <Toggle
                    label={t("fieldForm.required")}
                    checked={draft.required}
                    onChange={(required) => patch({ required })}
                  />
                )}

                <Toggle
                  label={t("fieldForm.unique")}
                  checked={draft.unique}
                  onChange={(unique) => patch({ unique })}
                />

                <Toggle
                  label={t("fieldForm.readonly")}
                  checked={draft.readonly}
                  onChange={(readonly) => patch({ readonly })}
                />

                {/*
                  Мультиязычность у существующего поля только СНИМАЕТСЯ.
                  Включать её здесь нечем: языковые колонки шлюз заводит
                  единственный раз, при создании (handlers/v2/field.go:84),
                  а PUT их не создаёт — флаг встал бы, а полей `title_en`
                  и `title_cyr` не появилось. Старая админка предлагала
                  этот переключатель всем, и он молча не работал.
                */}
                {MULTILANGUAGE_TYPES.has(draft.type) && draft.multilanguage && (
                  <Toggle
                    label={t("fieldForm.multilanguage")}
                    checked={draft.multilanguage}
                    onChange={(multilanguage) => patch({ multilanguage })}
                  />
                )}

                {/*
                  Проверка ввода. Выражение проверяет только фронт: бэкенд
                  эти ключи не читает, для него attributes — свободный мешок.
                  Текст отказа спрашивается, только когда выражение задано:
                  без выражения ему нечего сопровождать.
                */}
                <div className="px-2 py-1">
                  <span className="mb-0.5 block text-2xs text-fg-muted">
                    {t("fieldForm.validation")}
                  </span>

                  <input
                    value={draft.validation}
                    placeholder="^\+998\d{9}$"
                    aria-label={t("fieldForm.validation")}
                    spellCheck={false}
                    onChange={(event) => patch({ validation: event.target.value })}
                    className={`h-7 w-full rounded-md border bg-surface px-2 font-mono text-xs text-fg outline-none ${
                      isValidPattern(draft.validation)
                        ? "border-border-strong focus:border-accent"
                        : "border-danger"
                    }`}
                  />

                  {draft.validation.trim() && (
                    <input
                      value={draft.validationMessage}
                      placeholder={t("fieldForm.validationMessage")}
                      aria-label={t("fieldForm.validationMessage")}
                      onChange={(event) => patch({ validationMessage: event.target.value })}
                      className="mt-1 h-7 w-full rounded-md border border-border-strong bg-surface px-2 text-xs text-fg outline-none focus:border-accent"
                    />
                  )}
                </div>

                {/* Выражение или агрегат — только у полей-формул,
                    у остальных FormulaSettings ничего не рисует. */}
                <FormulaSettings
                  draft={draft}
                  fields={fields}
                  relations={relations}
                  language={language}
                  onChange={patch}
                />

                {/* Иконка и функция кнопки. Монтируется только у BUTTON:
                    список функций — отдельный запрос, и остальным полям
                    он не нужен. */}
                {(draft.type === "BUTTON" || hasScannerSettings(draft.type)) && (
                  <ButtonSettings
                    draft={draft}
                    icon={draft.type === "BUTTON"}
                    onChange={patch}
                  />
                )}

                {/* Настройки, которые есть у одного типа: точка карты,
                    формат снимка, перекодирование видео, сканер.
                    У остальных типов блок не рисуется. */}
                <TypeSettings draft={draft} onChange={patch} />

                {/* Автозаполнение из связанной строки. У таблицы без
                    подходящих связей блок не рисуется. */}
                <AutofillSettings
                  draft={draft}
                  relations={relations}
                  language={language}
                  onChange={patch}
                />

                {/* Приставка к автономеру: `INV-000123`. Правится и у
                    заведённого поля — её подставляет бэкенд в момент
                    вставки строки, а не при создании последовательности,
                    поэтому новая приставка действует со следующей записи. */}
                {hasPrefix(draft.type) && (
                  <label className="flex h-8 items-center gap-2 px-2">
                    <span className="flex-1 truncate text-sm text-fg">{t("fieldForm.prefix")}</span>
                    <input
                      value={draft.prefix}
                      // Без дефиса: его бэкенд ставит сам («INV» → «INV-000123»).
                      placeholder="INV"
                      onChange={(event) => patch({ prefix: event.target.value })}
                      className="h-6 w-24 rounded-md border border-border-strong bg-surface px-1.5 text-xs text-fg outline-none focus:border-accent"
                    />
                  </label>
                )}

                {/*
                  Значение по умолчанию у новой записи. Подставляет его
                  фронт (features/item/model/cell-value → blankItem):
                  колонки под него у поля нет, бэкенд просто хранит ключ
                  в attributes и при вставке про него не знает.
                */}
                {hasDefaultValue(draft.type) && (
                  <div className="flex flex-col gap-1 px-2 py-1">
                    <span className="text-2xs text-fg-muted">{t("fieldForm.defaultValue")}</span>
                    <input
                      value={draft.defaultValue}
                      placeholder={t("fieldForm.defaultValuePlaceholder")}
                      onChange={(event) => patch({ defaultValue: event.target.value })}
                      className="h-7 w-full rounded-md border border-border-strong bg-surface px-2 text-sm text-fg outline-none focus:border-accent"
                    />
                  </div>
                )}

                {/*
                  Одно и то же `digit_number` у двух разных настроек.

                  У INCREMENT_ID это разрядность последовательности:
                  учитывается, когда заводится сама последовательность,
                  и у существующего поля правка ничего не переписывает
                  задним числом — поэтому спрашиваем только при создании.

                  У генерируемых значений это ДЛИНА, и её читают на каждой
                  вставке: правится и потом.
                */}
                {(hasLength(draft.type) || (draft.type === "INCREMENT_ID" && !editing)) && (
                  <label className="flex h-8 items-center gap-2 px-2">
                    <span className="flex-1 truncate text-sm text-fg">
                      {t(hasLength(draft.type) ? "fieldForm.valueLength" : "fieldForm.digits")}
                    </span>
                    <input
                      value={draft.digits}
                      inputMode="numeric"
                      placeholder={hasLength(draft.type) ? String(DEFAULT_LENGTH) : "9"}
                      onChange={(event) => patch({ digits: event.target.value.replace(/\D/g, "") })}
                      className="h-6 w-12 rounded-md border border-border-strong bg-surface px-1.5 text-center text-xs tabular-nums text-fg outline-none focus:border-accent"
                    />
                  </label>
                )}

                {field && (
                  <>
                    <div className="my-1 h-px bg-border" />

                    <button
                      type="button"
                      // Панель закрывается ДО подтверждения: спрашивать
                      // «удалить?» из-под собственных настроек поля незачем,
                      // а сохранять черновик удаляемого поля — тем более.
                      onClick={() => {
                        onClose();
                        onDelete(field);
                      }}
                      className="flex h-8 w-full items-center gap-2 rounded-md px-2 text-left text-sm text-danger transition-colors hover:bg-danger-subtle"
                    >
                      <Icon as={IconTrash} size={16} />
                      <span className="truncate">{t("column.delete")}</span>
                    </button>
                  </>
                )}
              </div>
            )}
          </>
        )}
      </div>
    </Anchored>
  );
}

/**
 * Ссылка на вариант: стадия и номер в её списке. У MULTISELECT стадии
 * нет — список один (см. CONTEXT, FieldOption).
 */
type OptionRef = { group: StatusGroup | null; index: number };

/**
 * Экран одного варианта: имя, удаление и цвет.
 *
 * Отдельным экраном, а не строкой со всем сразу: у варианта три разные
 * вещи, и в строке шириной с панель они не помещаются — цвет
 * превращается в квадратик-загадку, а удаление стоит в пикселе от него.
 * Здесь же цвет виден списком с названиями, и нужный не приходится
 * угадывать по оттенку.
 */
function OptionScreen({
  option,
  onChange,
  onDelete,
  onBack,
}: {
  option: DraftOption;
  onChange: (next: Partial<DraftOption>) => void;
  onDelete: () => void;
  onBack: () => void;
}) {
  const { t } = useTranslation();

  return (
    <>
      <div className="flex items-center gap-1.5 p-1">
        <BackButton onClick={onBack} />

        <input
          autoFocus
          value={option.label}
          placeholder={t("fieldForm.optionPlaceholder")}
          aria-label={t("fieldForm.optionPlaceholder")}
          onChange={(event) => onChange({ label: event.target.value })}
          onKeyDown={(event) => {
            if (event.key !== "Enter") return;
            event.preventDefault();
            onBack();
          }}
          className="h-8 min-w-0 flex-1 rounded-md border border-border-strong bg-surface px-2 text-sm text-fg outline-none focus:border-accent"
        />
      </div>

      <button
        type="button"
        onClick={onDelete}
        className="flex h-8 w-full items-center gap-2 rounded-md px-2 text-left text-sm text-danger transition-colors hover:bg-danger-subtle"
      >
        <Icon as={IconTrash} size={16} />
        <span className="truncate">{t("action.delete")}</span>
      </button>

      <div className="my-1 h-px bg-border" />
      <p className="px-2 pb-1 text-2xs text-fg-muted">{t("fieldForm.optionColor")}</p>

      <div className="min-h-0 flex-1 overflow-y-auto">
        {CHIP_COLORS.map((color) => (
          <button
            key={color}
            type="button"
            onClick={() => onChange({ color })}
            className="flex h-8 w-full items-center gap-2 rounded-md px-2 text-left text-sm transition-colors hover:bg-surface-hover"
          >
            {/* Образец — сам чип: тот же оттенок, что встанет в ячейку,
                а не отдельный квадратик со своим набором токенов. */}
            <Chip color={color}>&nbsp;&nbsp;</Chip>
            <span className="flex-1 truncate">{t(`color.${color}`)}</span>
            {option.color === color && (
              <Icon as={IconCheck} size={14} className="shrink-0 text-accent-text" />
            )}
          </button>
        ))}
      </div>
    </>
  );
}

/** Возврат на предыдущий экран панели. Один вид у списка типов и у варианта. */
function BackButton({ onClick }: { onClick: () => void }) {
  const { t } = useTranslation();

  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={t("action.back")}
      title={t("action.back")}
      className="grid size-7 shrink-0 place-items-center rounded-md border border-border text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
    >
      <Icon as={IconChevronLeft} size={14} />
    </button>
  );
}

/**
 * Форма связи: создание и правка.
 *
 * Отдельный экран, а не строки в форме поля, и кнопка внизу вместо
 * клика по типу: у связи есть обязательная часть — целевая таблица, —
 * без которой создавать нечего. Остальные типы создаются кликом именно
 * потому, что кликом всё и сказано.
 *
 * Полей два, и это ровно то, без чего связь не работает: куда ведёт
 * и что показывать вместо uuid. Оба — выпадающие списки с поиском
 * и догрузкой по прокрутке: таблиц в проекте и полей в таблице бывают
 * сотни, и разворачивать их простынёй внутри узкой панели незачем.
 * Направление не спрашивается — оно одно, см. RELATION_DIRECTION. Остальные два десятка настроек из
 * старой админки (auto_filters, cascading, summaries, action_relations,
 * multiple_insert) не переносятся: их читает то, чего у нас пока нет.
 *
 * Целевую таблицу можно сменить и у существующей связи, но это НЕ
 * правка: связь пересоздаётся — старая удаляется, новая заводится.
 * Иначе никак. Имя колонки-ссылки выведено из целевой таблицы при
 * создании (relation.go:291 — `fieldFrom = TableTo + "_id"`), а Update
 * физическую колонку не трогает вовсе: связь начала бы утверждать одно,
 * а хранить другое. Цена — прежние связи строк, и о ней форма
 * предупреждает до нажатия, а не после.
 */
/**
 * Чем колонка-ссылка заполняется у новой записи.
 *
 * «Своя строка» — это не тот же человек, что «вошедший»: у курьера
 * есть id пользователя и есть строка в таблице курьеров, и связь
 * на «Курьера» должна получить вторую, а не первый (см. [[App Table]]).
 */
const SELF_DEFAULTS: { value: RelationDraft["selfDefault"]; labelKey: TranslationKey }[] = [
  { value: null, labelKey: "relationForm.selfNone" },
  { value: "user", labelKey: "relationForm.selfUser" },
  { value: "object", labelKey: "relationForm.selfObject" },
];

function RelationForm({
  draft,
  target,
  ourFields,
  language,
  onChange,
  onBack,
  onSubmit,
}: {
  draft: RelationDraft;
  /** Слаг целевой таблицы при правке. Пусто — связь заводится заново. */
  target: string;
  /** Поля ЭТОЙ таблицы: из них выбирается источник автофильтра. */
  ourFields: Field[];
  /** Язык ДАННЫХ: на нём подписаны таблицы проекта. */
  language: string;
  onChange: (draft: RelationDraft) => void;
  onBack: () => void;
  onSubmit: () => void;
}) {
  const { t } = useTranslation();
  const editing = Boolean(target);
  /** Целевую таблицу сменили — связь придётся пересоздать. */
  const retargeting = editing && draft.toSlug !== target;

  /*
   * Поиск у каждого списка свой: они ищут в разном — один по таблицам
   * проекта, другой по полям выбранной таблицы, — и общая строка
   * означала бы, что набранное в одном фильтрует другой.
   */
  const [tableSearch, setTableSearch] = useState("");
  const [fieldSearch, setFieldSearch] = useState("");

  const tables = useTables(tableSearch);
  const fields = useTableFields(draft.toSlug || undefined, fieldSearch);

  const patch = (next: Partial<RelationDraft>) => onChange({ ...draft, ...next });

  const toggleField = (id: string) =>
    patch({
      viewFieldIds: draft.viewFieldIds.includes(id)
        ? draft.viewFieldIds.filter((item) => item !== id)
        : [...draft.viewFieldIds, id],
    });

  return (
    <>
      <div className="mx-1 flex h-7 items-center gap-1.5 border-b border-border px-1 text-fg-subtle">
        <button
          type="button"
          onClick={onBack}
          aria-label={t("action.back")}
          className="grid size-5 shrink-0 place-items-center rounded transition-colors hover:bg-surface-hover hover:text-fg"
        >
          <Icon as={IconChevronLeft} size={14} />
        </button>
        <span className="flex-1 text-2xs">
          {t(editing ? "relationForm.editTitle" : "relationForm.title")}
        </span>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto p-1">
        <input
          autoFocus
          value={draft.label}
          placeholder={t("fieldForm.namePlaceholder")}
          aria-label={t("fieldForm.label")}
          onChange={(event) => patch({ label: event.target.value })}
          className="mb-2 h-8 w-full rounded-md border border-border-strong bg-surface px-2 text-sm text-fg outline-none focus:border-accent"
        />

        <SelectMenu
          label={t("relationForm.table")}
          placeholder={t("relationForm.pickTable")}
          searchPlaceholder={t("relationForm.searchTable")}
          emptyText={t("relationForm.noTables")}
          items={tables.items.map((table) => ({
            value: table.slug,
            label: localized(table.labels, language, table.label),
            icon: table.icon,
          }))}
          selected={new Set(draft.toSlug ? [draft.toSlug] : [])}
          search={tableSearch}
          loading={tables.isLoading}
          hasMore={tables.hasMore}
          onSearch={setTableSearch}
          onLoadMore={tables.loadMore}
          // Смена таблицы сбрасывает выбранные поля показа: они
          // принадлежали прежней, и их id в новой ничего не значат.
          onPick={(slug) => patch({ toSlug: slug, viewFieldIds: [] })}
        />

        {/* Смена таблицы у существующей связи — это пересоздание, и цена
            у него есть. Сказать о ней надо здесь, а не в подтверждении
            после нажатия. */}
        {retargeting && (
          <p className="m-1 rounded-md border border-danger bg-danger-subtle p-2 text-2xs text-danger">
            {t("relationForm.retargetWarning", { from: target })}
          </p>
        )}

        {draft.toSlug && (
          <div className="mt-2">
            <SelectMenu
              multiple
              label={t("relationForm.viewFields")}
              placeholder={t("relationForm.pickViewFields")}
              searchPlaceholder={t("table.searchField")}
              emptyText={t("table.noFields")}
              items={fields.items.map((field) => ({ value: field.id, label: field.label }))}
              selected={new Set(draft.viewFieldIds)}
              search={fieldSearch}
              loading={fields.isLoading}
              hasMore={fields.hasMore}
              onSearch={setFieldSearch}
              onLoadMore={fields.loadMore}
              onPick={toggleField}
            />

            {/* Ничего не выбрано — показывать вместо идентификатора нечего.
                Подставлять «первое текстовое» нельзя: это угадывание,
                см. [[Relation]] в CONTEXT. */}
            <p className="px-1 pt-1 text-2xs text-fg-subtle">
              {t("relationForm.viewFieldsHint")}
            </p>

            {/* Чем ограничен выбор: пары «поле этой строки → поле чужой
                таблицы». Настройка снизу, а не сверху, потому что её
                задают редко, а поля показа — всегда. */}
            <div className="mt-3 border-t border-border pt-2">
              <p className="px-1 text-2xs text-fg-muted">{t("relationForm.autoFilters")}</p>
              <p className="px-1 pb-1 text-2xs text-fg-subtle">
                {t("relationForm.autoFiltersHint")}
              </p>

              {draft.autoFilters.map((pair, index) => (
                <AutoFilterRow
                  key={index}
                  pair={pair}
                  ourFields={ourFields}
                  toSlug={draft.toSlug}
                  onChange={(next) =>
                    patch({
                      autoFilters: draft.autoFilters.map((item, at) =>
                        at === index ? next : item,
                      ),
                    })
                  }
                  onRemove={() =>
                    patch({ autoFilters: draft.autoFilters.filter((_, at) => at !== index) })
                  }
                />
              ))}

              <button
                type="button"
                onClick={() =>
                  patch({ autoFilters: [...draft.autoFilters, { fieldFrom: "", fieldTo: "" }] })
                }
                className="flex h-7 w-full items-center gap-1.5 rounded-md px-2 text-2xs text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
              >
                <Icon as={IconPlus} size={12} />
                {t("relationForm.addAutoFilter")}
              </button>
            </div>

            {/* Чем колонка заполнена у НОВОЙ записи. Три состояния, а не
                два флажка: в старой админке их два и поднять можно оба,
                хотя значение всё равно уедет одно. */}
            <div className="mt-3 border-t border-border pt-2">
              <p className="px-1 text-2xs text-fg-muted">{t("relationForm.selfDefault")}</p>
              <p className="px-1 pb-1 text-2xs text-fg-subtle">
                {t("relationForm.selfDefaultHint")}
              </p>

              {SELF_DEFAULTS.map(({ value, labelKey }) => (
                <label
                  key={labelKey}
                  className="flex h-7 cursor-pointer items-center gap-2 rounded-md px-2 text-2xs text-fg transition-colors hover:bg-surface-hover"
                >
                  <input
                    type="radio"
                    name="self-default"
                    checked={draft.selfDefault === value}
                    onChange={() => patch({ selfDefault: value })}
                    className="accent-accent"
                  />
                  {t(labelKey)}
                </label>
              ))}
            </div>
          </div>
        )}
      </div>

      <button
        type="button"
        disabled={!isRelationReady(draft)}
        onClick={onSubmit}
        /* Опасное действие красится так же, как в диалоге подтверждения:
           текст цветом danger на своей подложке, а не залитая кнопка. */
        className={`m-1 h-8 shrink-0 rounded-md text-sm transition-opacity hover:opacity-90 disabled:opacity-40 ${
          retargeting
            ? "border border-danger bg-danger-subtle text-danger"
            : "bg-accent text-accent-fg"
        }`}
      >
        {t(retargeting ? "relationForm.retarget" : editing ? "action.save" : "action.create")}
      </button>
    </>
  );
}

/**
 * Одно условие автофильтра: поле ЭТОЙ строки слева, поле чужой таблицы
 * справа.
 *
 * Списки разные по природе, отсюда и два разных поиска: свои поля уже
 * загружены и отсеиваются на месте, чужие приходится спрашивать —
 * их в таблице бывают сотни. Свой `useTableFields` у каждой строки,
 * а не один на форму, ровно поэтому: набранное в одной строке не должно
 * фильтровать соседнюю.
 */
function AutoFilterRow({
  pair,
  ourFields,
  toSlug,
  onChange,
  onRemove,
}: {
  pair: AutoFilterPair;
  ourFields: Field[];
  toSlug: string;
  onChange: (pair: AutoFilterPair) => void;
  onRemove: () => void;
}) {
  const { t } = useTranslation();
  const [ourSearch, setOurSearch] = useState("");
  const [theirSearch, setTheirSearch] = useState("");
  const theirs = useTableFields(toSlug, theirSearch);

  const needle = ourSearch.trim().toLowerCase();
  const ours = ourFields
    .filter((field) => !needle || `${field.label} ${field.slug}`.toLowerCase().includes(needle))
    .map((field) => ({ value: field.slug, label: field.label || field.slug }));

  return (
    <div className="flex items-end gap-1 px-1 pb-1">
      <div className="min-w-0 flex-1">
        <SelectMenu
          label={t("relationForm.autoFilterFrom")}
          placeholder={t("relationForm.pickField")}
          searchPlaceholder={t("table.searchField")}
          emptyText={t("table.noFields")}
          items={ours}
          selected={new Set(pair.fieldFrom ? [pair.fieldFrom] : [])}
          search={ourSearch}
          onSearch={setOurSearch}
          onLoadMore={() => {}}
          onPick={(slug) => onChange({ ...pair, fieldFrom: slug })}
        />
      </div>

      <div className="min-w-0 flex-1">
        <SelectMenu
          label={t("relationForm.autoFilterTo")}
          placeholder={t("relationForm.pickField")}
          searchPlaceholder={t("table.searchField")}
          emptyText={t("table.noFields")}
          /* Слаг, а не id: в `auto_filters` лежат слаги, и по ним же
             собирается запрос строк. */
          items={theirs.items.map((field) => ({ value: field.slug, label: field.label }))}
          selected={new Set(pair.fieldTo ? [pair.fieldTo] : [])}
          search={theirSearch}
          loading={theirs.isLoading}
          hasMore={theirs.hasMore}
          onSearch={setTheirSearch}
          onLoadMore={theirs.loadMore}
          onPick={(slug) => onChange({ ...pair, fieldTo: slug })}
        />
      </div>

      <button
        type="button"
        onClick={onRemove}
        aria-label={t("action.delete")}
        className="grid size-7 shrink-0 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-danger-subtle hover:text-danger"
      >
        <Icon as={IconTrash} size={14} />
      </button>
    </div>
  );
}

function Row({ label, value, onClick }: { label: string; value: string; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex h-8 w-full items-center gap-2 rounded-md px-2 text-left text-sm transition-colors hover:bg-surface-hover"
    >
      <span className="flex-1 truncate">{label}</span>
      <span className="truncate text-fg-muted">{value}</span>
      <Icon as={IconChevronRight} size={14} className="shrink-0 text-fg-subtle" />
    </button>
  );
}

/**
 * Список типов с иконками — теми же, что стоят в заголовках колонок:
 * человек выбирает то, что потом увидит в шапке.
 *
 * Иконка приходит снаружи параметром, а не импортом из features/item:
 * тот сам зависит от features/table, и импорт назад замкнул бы кольцо
 * между двумя index.ts.
 */
/** Переключатель настройки: подпись слева, флажок справа. */
function Toggle({
  label,
  checked,
  onChange,
}: {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
}) {
  return (
    <label className="flex h-8 cursor-pointer items-center gap-2 rounded-md px-2 transition-colors hover:bg-surface-hover">
      <span className="flex-1 truncate text-sm text-fg">{label}</span>
      <Checkbox checked={checked} onChange={(event) => onChange(event.target.checked)} />
    </label>
  );
}

function TypeList({
  value,
  icon,
  onPick,
  onBack,
  autoFocus,
  withRelation,
}: {
  value: string;
  icon: (type: string) => TablerIcon;
  onPick: (type: string) => void;
  /** Вернуться, не меняя тип. null — возвращаться некуда (новое поле). */
  onBack: (() => void) | null;
  autoFocus: boolean;
  /**
   * Показывать ли «Связь». Заводить её умеет не всякий вызывающий:
   * у связи своя ручка, и без обработчика выбор ничем не кончится.
   * Существующему полю смена типа на связь недоступна вовсе — связь
   * не тип колонки, а отдельная сущность.
   */
  withRelation: boolean;
}) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");
  const list = useRef<HTMLDivElement>(null);
  const current = useRef<HTMLButtonElement>(null);

  /*
   * Список открывается на выбранном типе. Прокрутка присваиванием,
   * а не scrollIntoView: тот тащит за собой всю панель.
   *
   * У контейнера обязателен `relative`: offsetTop считается от
   * ближайшего ПОЗИЦИОНИРОВАННОГО предка, и без него это координата
   * внутри всей страницы — список уезжает в случайное место.
   */
  useEffect(() => {
    const container = list.current;
    const button = current.current;
    if (!container || !button) return;

    container.scrollTop = button.offsetTop - container.clientHeight / 2 + button.clientHeight / 2;
  }, []);

  /*
   * Типов в ucode больше, чем мы предлагаем заводить (FORMULA, DYNAMIC,
   * BUTTON и прочее наследство). У существующего поля тип может быть
   * как раз из тех: показываем его отдельной строкой, иначе список
   * выглядит так, будто ничего не выбрано.
   */
  const offered = withRelation
    ? FIELD_TYPE_GROUPS
    : FIELD_TYPE_GROUPS.filter((group) => group.key !== "relation");

  const known = offered.some((group) => group.types.some((item) => item.type === value));
  const all: FieldTypeGroup[] = known
    ? offered
    : [{ key: "current", types: [{ type: value, label: fieldTypeLabel(value) }] }, ...offered];

  const needle = query.trim().toLowerCase();
  const matched = all
    .map((group) => group.types.filter((item) => item.label.toLowerCase().includes(needle)))
    .filter((types) => types.length > 0);

  const blocks = toBlocks(matched);
  const items = blocks.flat();

  return (
    <>
      <div className="mx-1 flex h-7 items-center gap-1.5 border-b border-border px-1 text-fg-subtle">
        {onBack ? (
          <button
            type="button"
            onClick={onBack}
            aria-label={t("action.back")}
            title={t("action.back")}
            className="grid size-5 shrink-0 place-items-center rounded-md transition-colors hover:bg-surface-hover hover:text-fg"
          >
            <Icon as={IconChevronLeft} size={14} />
          </button>
        ) : (
          <Icon as={IconSearch} size={14} />
        )}

        <input
          autoFocus={autoFocus}
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          onKeyDown={(event) => {
            // Набрал «num» — Enter берёт первый подошедший тип. Иначе
            // поиск сужает список до одной строки, до которой всё равно
            // надо тянуться мышью.
            const first = items[0];
            if (event.key === "Enter" && first) {
              event.preventDefault();
              onPick(first.type);
            }
          }}
          placeholder={t("fieldForm.selectType")}
          className="h-full min-w-0 flex-1 bg-transparent text-xs text-fg outline-none placeholder:text-fg-subtle"
        />
      </div>

      {/* Высота под весь список: в две колонки он ровно шестнадцать строк,
          и прокрутка за ними — это прокрутка ради одной. Панель целиком
          всё равно ограничена 70vh, так что на низком экране обрежется она. */}
      <div ref={list} className="relative max-h-[27rem] overflow-y-auto">
        {blocks.map((block, index) => (
          <div key={block[0]?.type ?? index}>
            {index > 0 && <div className="mx-2 my-1 h-px bg-border" />}

            <div className="grid grid-cols-2">
              {block.map((item) => (
                <button
                  key={item.type}
                  ref={item.type === value ? current : undefined}
                  type="button"
                  onClick={() => onPick(item.type)}
                  className={`flex h-7 w-full items-center gap-2 rounded-md px-2 text-left text-sm transition-colors ${
                    item.type === value
                      ? "bg-accent-subtle text-accent-text"
                      : "hover:bg-surface-hover"
                  }`}
                >
                  <Icon as={icon(item.type)} size={14} className="shrink-0 text-fg-muted" />
                  <span className="truncate">{item.label}</span>
                </button>
              ))}
            </div>
          </div>
        ))}

        {!items.length && <p className="px-2 py-2 text-xs text-fg-subtle">{t("table.noOptions")}</p>}
      </div>
    </>
  );
}

/** Кнопка «+» у заголовка списка вариантов. */
function AddOption({ onClick }: { onClick: () => void }) {
  const { t } = useTranslation();

  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={t("fieldForm.addOption")}
      title={t("fieldForm.addOption")}
      className="grid size-5 shrink-0 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-surface-hover hover:text-fg"
    >
      <Icon as={IconPlus} size={14} />
    </button>
  );
}

/**
 * Варианты выбора списком: чип таким, каким он встанет в ячейку.
 *
 * Строка открывает вариант отдельным экраном (OptionScreen), а не правит
 * его на месте: имя, цвет и удаление в одну строку панели не помещаются.
 *
 * Пустые варианты не сохраняются (api/fields их отсеивает), поэтому
 * отдельной кнопки «отменить добавление» здесь нет.
 */
function OptionList({
  options,
  onOpen,
}: {
  options: DraftOption[];
  onOpen: (index: number) => void;
}) {
  const { t } = useTranslation();

  return (
    <div className="flex flex-col">
      {options.map((option, index) => (
        <button
          key={index}
          type="button"
          onClick={() => onOpen(index)}
          className="flex h-8 w-full items-center gap-2 rounded-md px-1.5 text-left transition-colors hover:bg-surface-hover"
        >
          {/* Безымянный вариант — это тот, который только что завели
              и не дописали: подпись-подсказка честнее пустого чипа. */}
          <Chip color={option.color}>
            {option.label.trim() || t("fieldForm.optionPlaceholder")}
          </Chip>

          <Icon as={IconChevronRight} size={14} className="ml-auto shrink-0 text-fg-subtle" />
        </button>
      ))}
    </div>
  );
}
