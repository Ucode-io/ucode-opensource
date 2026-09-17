import { Fragment, useDeferredValue, useState } from "react";
import { IconChevronDown, IconChevronUp, IconFileSpreadsheet } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";
import { DatePicker } from "@/shared/ui/date-picker";
import { Icon } from "@/shared/ui/icon";
import { Input } from "@/shared/ui/input";
import { Tabs } from "@/shared/ui/tabs";
import {
  ACTIVITY_PAGE,
  NO_FILTERS,
  useActivity,
  useActivityEntry,
  useExportActivity,
  type ActivityFilters,
} from "../api/activity";
import { diffEntry, unwrapEntry, type EntryChange } from "../model/activity";
import { relativeTime } from "../model/time";
import { FunctionLogs } from "./FunctionLogs";
import { ActionBadge, Empty, Pager, SectionHeader, Td, Th, formatDateTime } from "./parts";
import { TableFilter } from "./TableFilter";
import { Usage } from "./Usage";

/**
 * Журнал изменений: кто, когда и что поменял.
 *
 * Запись хранит «было» и «стало», поэтому открытая строка отвечает
 * не только «кто трогал таблицу», но и «что именно в ней стало другим»
 * — ради этого журнал и читают.
 *
 * Отбор по подстроке, а не выбором из списка: типов действий в базе
 * четыре десятка (`CREATE ITEM`, `UPDATE FIELD`, `DELETE MENU`…),
 * список растёт вместе с ручками бэкенда, и зашитый в код перечень
 * устарел бы молча — так же, как он устарел в старой админке.
 * Сервер и сам сравнивает их через ILIKE.
 *
 * Рядом — вкладка выполнения функций. Предмет у неё другой (как
 * отработал вызов, а не что поменялось в проекте), и общих колонок
 * с этим списком нет ни одной, кроме даты, — поэтому вкладка,
 * а не строки вперемешку. См. FunctionLogs.
 *
 * Третья вкладка — расход API-лимита: тоже журнал активности проекта,
 * только просуммированный по маршрутам. См. Usage.
 */
export function ActivityLog() {
  const { t } = useTranslation();
  const [tab, setTab] = useState("changes");

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <div className="shrink-0 border-b border-border px-4 py-2">
        <Tabs
          tabs={[
            { id: "changes", label: t("activity.tabChanges") },
            { id: "functions", label: t("activity.tabFunctions") },
            { id: "usage", label: t("activity.tabUsage") },
          ]}
          activeId={tab}
          onSelect={setTab}
        />
      </div>

      {tab === "changes" && <ChangesLog />}
      {tab === "functions" && <FunctionLogs />}
      {tab === "usage" && <Usage />}
    </div>
  );
}

function ChangesLog() {
  const { t, i18n } = useTranslation();

  const [filters, setFilters] = useState<ActivityFilters>(NO_FILTERS);
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(ACTIVITY_PAGE);

  /*
   * Текстовые поля отложены, даты — нет: дату вводят целиком и разом,
   * а буквы по одной. Ключ запроса собирается из отложенного значения,
   * поэтому лишних запросов на каждую букву не уходит.
   */
  const deferred = useDeferredValue(filters);
  const { entries, count, isLoading } = useActivity(deferred, page, limit);

  const [opened, setOpened] = useState("");
  const exportExcel = useExportActivity();

  const put = (patch: Partial<ActivityFilters>) => {
    setFilters((current) => ({ ...current, ...patch }));
    setPage(1);
  };

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <SectionHeader title={t("activity.title")} hint={t("activity.hint")}>
        {/* Выгружается отобранное, а не страница: файл делают, чтобы
            посмотреть шире экрана.

            Кнопка основная, как и в остальных разделах настроек: это
            единственное действие в шапке. `secondary` здесь белая
            на белой панели и держится на одном волоске границы —
            вариант рассчитан на фон приложения, а не на поверхность. */}
        <Button
          size="sm"
          disabled={exportExcel.isPending}
          onClick={() => exportExcel.mutate(deferred)}
        >
          <Icon as={IconFileSpreadsheet} size={14} />
          {exportExcel.isPending ? t("common.loading") : t("activity.export")}
        </Button>
      </SectionHeader>

      {/*
        Ширину задаёт обёртка, а не само поле: у `Input` в базовых
        классах стоит `w-full`, и своя ширина на нём — спор двух
        одинаковых по весу правил, который выигрывает не тот, кто
        написан последним.
      */}
      <div className="flex shrink-0 flex-wrap items-end gap-2 border-b border-border px-4 py-2">
        <div className="w-40">
          <Input
            value={filters.action}
            onChange={(event) => put({ action: event.target.value })}
            placeholder={t("activity.action")}
            aria-label={t("activity.action")}
            className="h-7 text-sm"
          />
        </div>

        <div className="w-40">
          <TableFilter value={filters.table} onChange={(table) => put({ table })} />
        </div>

        <div className="w-40">
          <Input
            value={filters.user}
            onChange={(event) => put({ user: event.target.value })}
            placeholder={t("activity.user")}
            aria-label={t("activity.user")}
            className="h-7 text-sm"
          />
        </div>

        {/* Наш календарь, а не нативное поле: у `<input type="date">`
            свой вид в каждой системе и светлый календарь в тёмной теме. */}
        <div className="w-36">
          <DatePicker
            value={filters.from}
            locale={i18n.language}
            placeholder={t("activity.from")}
            ariaLabel={t("activity.from")}
            clearLabel={t("table.clearFilters")}
            onChange={(from) => put({ from })}
            className="h-7"
          />
        </div>

        <div className="w-36">
          <DatePicker
            value={filters.to}
            locale={i18n.language}
            placeholder={t("activity.to")}
            ariaLabel={t("activity.to")}
            clearLabel={t("table.clearFilters")}
            onChange={(to) => put({ to })}
            className="h-7"
          />
        </div>

        {/* Кнопка появляется по заполненности, а не по «трогали ли»:
            стёртое поле — это тот же пустой отбор. */}
        {Object.values(filters).some(Boolean) && (
          <Button
            size="sm"
            variant="ghost"
            onClick={() => {
              setFilters(NO_FILTERS);
              setPage(1);
            }}
          >
            {t("table.clearFilters")}
          </Button>
        )}
      </div>

      <div className="min-h-0 flex-1 overflow-auto">
        <table className="w-full border-separate border-spacing-0">
          <thead>
            <tr>
              <Th className="w-44">{t("activity.date")}</Th>
              <Th>{t("activity.action")}</Th>
              <Th>{t("activity.table")}</Th>
              <Th>{t("activity.user")}</Th>
              <Th className="w-24">{t("activity.status")}</Th>
              <Th className="w-10" />
            </tr>
          </thead>

          <tbody>
            {isLoading && <Empty text={t("common.loading")} colSpan={6} />}
            {!isLoading && !entries.length && <Empty text={t("activity.empty")} colSpan={6} />}

            {entries.map((entry) => (
              /*
               * Строка и её раскрытие — соседи, а не вложенные: в таблице
               * подробности живут своей `<tr>` во всю ширину. Поэтому
               * фрагмент с ключом, а не обёртка: лишний узел между
               * `<tbody>` и `<tr>` разметку таблицы ломает.
               */
              <Fragment key={entry.id}>
                <tr
                  onClick={() => setOpened(opened === entry.id ? "" : entry.id)}
                  /* Открытая строка — `surface-active`, а не `surface-hover`:
                     наведением подсвечивается любая, и одинаковый тон
                     не отличал бы открытую от той, под которой сейчас
                     курсор. Это та же пара тонов, что у списка
                     подключений. */
                  className={`cursor-pointer transition-colors hover:bg-surface-hover ${
                    opened === entry.id ? "bg-surface-active" : ""
                  }`}
                >
                  <Td className="text-fg-muted">
                    {formatDateTime(entry.date, i18n.language)}
                    {/* Вторая строка отвечает на другой вопрос: не «когда
                        именно», а «давно ли». Считать это в уме из даты
                        человек не должен. */}
                    <span className="block text-2xs text-fg-subtle">
                      {relativeTime(entry.date, i18n.language)}
                    </span>
                  </Td>
                  <Td>
                    <ActionBadge action={entry.action} />
                  </Td>
                  <Td className="text-fg-muted">{entry.table}</Td>
                  <Td className="text-fg-muted">{entry.user}</Td>
                  <Td>
                    {/*
                      Метод здесь тихим текстом, а не плашкой, как
                      в журнале функций: рядом уже стоит плашка
                      действия, и она говорит то же самое — `UPDATE`
                      это и есть `PUT`. Две цветные метки в одной
                      строке об одном и том же — это не «заметнее»,
                      а «пестрее»; цвет достаётся той, что несёт
                      и глагол, и сущность.

                      Код ответа рисуется только тогда, когда он есть:
                      у записей до появления колонки он нулевой, и «0»
                      читалось бы как настоящий ответ. Успех НЕ красим
                      в зелёный: двухсотых подавляющее большинство,
                      и зелёная стена перестаёт что-либо выделять —
                      глаз ищет здесь отказ.
                    */}
                    <span className="flex items-baseline gap-1.5 font-mono text-2xs text-fg-subtle">
                      {entry.method}

                      {entry.statusCode > 0 && (
                        <span
                          className={`tabular-nums ${
                            entry.statusCode < 400 ? "text-fg-muted" : "text-danger"
                          }`}
                        >
                          {entry.statusCode}
                        </span>
                      )}
                    </span>
                  </Td>

                  <Td className="text-right">
                    {/* Кнопка, а не один значок на строке: строку мышью
                        открывают целиком, но с клавиатуры до неё иначе
                        не добраться — `<tr>` фокус не принимает.
                        `stopPropagation` — чтобы нажатие не сосчиталось
                        дважды: сначала кнопкой, потом строкой под ней. */}
                    <button
                      type="button"
                      aria-expanded={opened === entry.id}
                      aria-controls={`entry-${entry.id}`}
                      aria-label={opened === entry.id ? t("tree.collapse") : t("tree.expand")}
                      onClick={(event) => {
                        event.stopPropagation();
                        setOpened(opened === entry.id ? "" : entry.id);
                      }}
                      /* `cursor-pointer` явно: у `<button>` курсор по
                         умолчанию стрелка, а preflight Tailwind v4
                         его больше не переопределяет. */
                      className="grid size-6 cursor-pointer place-items-center rounded text-fg-subtle transition-colors hover:bg-surface-active hover:text-fg"
                    >
                      <Icon
                        as={IconChevronDown}
                        size={14}
                        className={`transition-transform ${opened === entry.id ? "rotate-180" : ""}`}
                      />
                    </button>
                  </Td>
                </tr>

                {opened === entry.id && (
                  <tr>
                    {/*
                      Подробности лежат под своей строкой, а не поверх
                      списка: соседние записи остаются видны, и чтобы
                      посмотреть следующую, окно не надо закрывать.

                      Полоса акцента слева связывает раскрытое с его
                      строкой: без неё это просто серый прямоугольник
                      между двумя записями, и к какой из них он
                      относится — к той, что выше, или к той, что ниже,
                      — приходится догадываться. Цвет тот же, которым
                      в системе помечено активное (`docs/DESIGN.md`).
                    */}
                    <td
                      colSpan={6}
                      className="border-b border-b-border border-l-2 border-l-accent bg-bg px-4 py-3"
                    >
                      {/*
                        Появление — та же анимация, что у всего, что
                        открывается в этом приложении (`--animate-page`,
                        180мс ease-out): высоту строки таблицы плавно
                        не разогнать, а мгновенная подмена содержимого
                        под курсором читается как подёргивание. При
                        `prefers-reduced-motion` она гасится глобально.
                      */}
                      <div id={`entry-${entry.id}`} className="animate-page">
                        <EntryDetails id={entry.id} />
                      </div>
                    </td>
                  </tr>
                )}
              </Fragment>
            ))}
          </tbody>
        </table>
      </div>

      <Pager
        page={page}
        total={count}
        limit={limit}
        onPage={setPage}
        onLimit={(next) => {
          setLimit(next);
          // Пятая страница по двадцать — это не пятая по сотне: после
          // смены размера прежний номер указывает в другое место.
          setPage(1);
        }}
      />
    </div>
  );
}

/**
 * Подробности записи — под самой записью, а не поверх списка.
 *
 * Модальное окно здесь было не на месте: журнал читают строку за
 * строкой, а окно накрывало список целиком, и ради соседней записи его
 * приходилось закрывать. Раскрытая строка оставляет соседей на виду.
 * Так же сделано у ugen (`logs-view.tsx`, аккордеон), и это
 * единственное, что оттуда стоило взять: содержимое у них — тот же
 * сырой JSON в трёх вкладках.
 *
 * Сверху только разошедшиеся поля, каждое строкой. «Было» и «стало»
 * целиком — два полотна по две сотни строк, отличающиеся номером
 * порядка да одним идентификатором; открывали запись ради этого
 * отличия, а находить его приходилось глазами. Полотна остались,
 * но под кнопкой — для случая, когда нужно свериться с ответом
 * целиком.
 *
 * Пустые разделы не рисуются вовсе: у записи о чтении нет ни «было»,
 * ни «стало», и четыре подписи с прочерками только мешают.
 */
function EntryDetails({ id }: { id: string }) {
  const { t } = useTranslation();
  const { entry, isLoading } = useActivityEntry(id);

  const [raw, setRaw] = useState(false);

  const changes = entry ? diffEntry(entry.before, entry.after) : [];

  /*
   * Повторы выброшены, и это не косметика.
   *
   * Шлюз кладёт ОДНО И ТО ЖЕ значение в два поля: `logReq.Response = resp`
   * и `logReq.Current = resp` стоят рядом в каждом обработчике правки
   * (`api/handlers/v2/view.go:294`, `field.go:164`, `relation.go:253`
   * и ещё десяток мест). То есть «Ответ» — это буква в букву «Стало».
   *
   * А «Запрос» у правки — тело запроса, то есть тот же самый объект
   * сущности (`Request: &view`), поэтому он и выглядит как ответ. Это
   * правда о записи, а не ошибка показа: убрать его нельзя, он всё же
   * другой — в ответе заполнены поля, которые проставил сервер.
   *
   * Остаётся первый по порядку, а порядок здесь от важного к служебному.
   */
  const payloads = entry
    ? ([
        ["activity.before", unwrapEntry(entry.before)],
        ["activity.after", unwrapEntry(entry.after)],
        ["activity.request", unwrapEntry(entry.request)],
        ["activity.response", unwrapEntry(entry.response)],
      ] as const).filter(([, value]) => value)
    : [];

  const parts = payloads.filter(
    ([, value], index) => payloads.findIndex(([, other]) => other === value) === index,
  );

  return (
    <div className="space-y-3">
      {isLoading && <p className="text-sm text-fg-muted">{t("common.loading")}</p>}

      {/* Длительность — единственное, чего нет в самой строке. */}
      {entry?.duration ? (
        <p className="text-2xs text-fg-subtle tabular-nums">{entry.duration} ms</p>
      ) : null}

      {!isLoading && !parts.length && (
        <p className="text-sm text-fg-subtle">{t("activity.noPayload")}</p>
      )}

      {changes.length > 0 && (
        <div className="overflow-hidden rounded-md border border-border bg-surface">
          {changes.map((change) => (
            <div
              key={change.path}
              className="flex flex-col gap-1 border-b border-border px-3 py-2 last:border-b-0 sm:flex-row sm:gap-3"
            >
              {/* Путь — слева постоянной ширины: список читается
                  колонкой имён, а не лесенкой. */}
              <p className="shrink-0 truncate font-mono text-xs text-fg-muted sm:w-52">
                {change.path}
              </p>

              <div className="min-w-0 flex-1">
                <Change change={change} />
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Поля есть, а различий нет: правка ничего не изменила
          (такое пишется в журнал), — и это ответ, а не пустой экран. */}
      {!isLoading && !changes.length && parts.length > 0 && (
        <p className="text-sm text-fg-subtle">{t("activity.noChanges")}</p>
      )}

      {parts.length > 0 && (
        <div>
          <Button size="sm" variant="ghost" onClick={() => setRaw(!raw)}>
            <Icon as={raw ? IconChevronUp : IconChevronDown} size={14} />
            {raw ? t("activity.hideRaw") : t("activity.showRaw")}
          </Button>
        </div>
      )}

      {raw &&
        parts.map(([key, value]) => (
          <section key={key}>
            <h3 className="mb-1 text-xs font-medium text-fg-muted">{t(key)}</h3>
            {/* Своя горизонтальная прокрутка: длинная строка JSON
                иначе растягивает строку таблицы шире экрана. */}
            <pre className="max-h-64 overflow-auto rounded-md bg-surface p-3 font-mono text-xs text-fg">
              {value}
            </pre>
          </section>
        ))}
    </div>
  );
}

/**
 * Что стало с одним полем.
 *
 * Короткое значение — строкой «было → стало»: две рамки ради «10 → 20»
 * занимают полэкрана, а сравнивать там нечего. Длинное всё-таки
 * разводится в два столбца — в строку оно не читается.
 *
 * Цветом помечены только знаки списка: плюс и минус — это добавили
 * и убрали, то есть ровно то, что означают success и danger. Обычная
 * смена значения цвета не получает: направление несёт стрелка, и
 * красить каждую правку значило бы раскрасить весь журнал.
 */
function Change({ change }: { change: EntryChange }) {
  const { t } = useTranslation();

  if ("added" in change) {
    // Набор тот же — значит переставили. Печатать список дважды незачем:
    // сам факт и есть всё содержание правки.
    if (!change.added.length && !change.removed.length) {
      return <p className="text-xs text-fg-subtle">{t("activity.reordered")}</p>;
    }

    return (
      <ul className="max-h-40 space-y-0.5 overflow-auto">
        {change.removed.map((item, index) => (
          <li key={`-${index}`} title={item} className="truncate font-mono text-xs text-fg">
            <span className="text-danger">−</span> {item}
          </li>
        ))}

        {change.added.map((item, index) => (
          <li key={`+${index}`} title={item} className="truncate font-mono text-xs text-fg">
            <span className="text-success">+</span> {item}
          </li>
        ))}
      </ul>
    );
  }

  if (inline(change.before) && inline(change.after)) {
    return (
      <p className="flex items-baseline gap-1.5 font-mono text-xs">
        <span className="min-w-0 truncate text-fg-muted">{change.before || "—"}</span>
        <span className="shrink-0 text-fg-subtle">→</span>
        <span className="min-w-0 truncate text-fg">{change.after || "—"}</span>
      </p>
    );
  }

  return (
    <div className="grid gap-2 sm:grid-cols-2">
      <Side label={t("activity.before")} value={change.before} />
      <Side label={t("activity.after")} value={change.after} />
    </div>
  );
}

/** Влезает в строку: без переносов и не длиннее половины ширины. */
function inline(value: string): boolean {
  return value.length <= 48 && !value.includes("\n");
}

/**
 * Одна сторона изменения. Пусто — это «поля не было», и так и написано
 * прочерком: пустая рамка читалась бы как «не загрузилось».
 */
function Side({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <p className="mb-0.5 text-2xs text-fg-subtle">{label}</p>
      <pre className="max-h-40 overflow-auto rounded-md bg-bg p-2 font-mono text-xs text-fg">
        {value || "—"}
      </pre>
    </div>
  );
}
