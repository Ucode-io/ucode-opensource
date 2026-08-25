import { useParams } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { useTableSchema } from "@/features/table";
import { useDataLanguages } from "@/features/workspace";
import { Modal } from "@/shared/ui/modal";
import { useItem } from "../api/items";
import { useDrawerLayout } from "../api/layout";
import { ItemDrawer } from "./ItemDrawer";

/**
 * Карточка записи ЧУЖОЙ таблицы, открытая прямо из поля-связи.
 *
 * Раньше добраться до связанной записи можно было только вкладкой
 * связи — а вкладку заводит админ. Здесь тот же переход делается
 * из самого значения: у выбранной строки есть стрелка.
 *
 * **Своего адреса у этой карточки нет, и это решение, а не упущение.**
 * Наш адрес — `/m/<пункт меню>?item=<guid>`, а у чужой таблицы пункта
 * меню может не быть вовсе: связь ведёт куда угодно, в том числе
 * в справочник, которого нет в сайдбаре. Придуманный адрес
 * (`?related=<слаг>:<guid>`) при открытии по холодной ссылке оставил
 * бы карточку без пункта меню — то есть без раскладки и без прав, —
 * и потребовал бы второго способа их разрешать. Поэтому карточка
 * живёт поверх и закрывается туда, откуда её открыли, как и карточка
 * внутри вкладки связи.
 *
 * Раскладка спрашивается по НАШЕМУ пункту меню: своего у чужой таблицы
 * нет, а ручка раскладки требует пары «таблица + пункт». Так же делает
 * и вкладка связи.
 *
 * Всё остальное компонент берёт сам — пункт меню из адреса, языки
 * данных из проекта: он открывается из ячейки, а протаскивать четыре
 * пропса через грид, доску и карточку ради одной стрелки незачем.
 */
export function ForeignRecord({
  tableSlug,
  guid,
  language,
  trail,
  onClose,
}: {
  tableSlug: string;
  guid: string;
  /** Язык ДАННЫХ: на нём читаются подписи полей чужой таблицы. */
  language: string;
  /** Путь до карточки: откуда её открыли. */
  trail: { label: string; onClick: () => void }[];
  onClose: () => void;
}) {
  const { t, i18n } = useTranslation();
  const params = useParams({ strict: false });
  const menuId = typeof params.menuId === "string" ? params.menuId : "";

  const { languages } = useDataLanguages();
  const { schema } = useTableSchema(tableSlug, []);
  const { item, isLoading, error } = useItem(tableSlug, guid, true);
  const layout = useDrawerLayout({ tableSlug, menuId, language });

  const columns = schema.fields.filter((field) => !layout.hidden.has(field.slug));

  return (
    /*
     * Подложка прозрачная: карточка рисует свою сама, когда открыта
     * по центру (`ItemDrawer`), а сбоку затемнения нет вовсе — так же,
     * как у карточки во вкладке связи. Иначе по центру их было бы две,
     * одна поверх другой.
     *
     * Само окно при этом нужно: оно и ловит щелчок мимо, и метит
     * поддерево как `data-modal` — по этой метке всплывашка над ячейкой,
     * из которой карточку открыли, не закрывается от её прокрутки
     * (см. shared/ui/anchored).
     */
    <Modal onClose={onClose} background="transparent">
      <ItemDrawer
        tableSlug={tableSlug}
        columns={columns}
        row={item}
        loading={isLoading}
        error={error ? t("drawer.notFound") : null}
        relations={schema.relations}
        locale={i18n.language}
        language={language}
        languages={languages}
        sections={layout.sections}
        heading={layout.heading}
        trail={trail}
        /*
         * Только чтение. Ручка правки та же самая, но правка ушла бы
         * в таблицу, которой на экране нет: список под нами показывает
         * НАШИ строки и о правке чужой не узнает. Открыли посмотреть —
         * смотрим.
         */
        onClose={onClose}
      />
    </Modal>
  );
}
