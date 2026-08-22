# Чек паритета со старой админкой

Как заполняется — [PLAN-v2.md](PLAN-v2.md), §5. Коротко: одна фича
за проход, поведение выписывается ИЗ КОДА старого экрана, каждая строка
получает статус. «Выброшено» без причины — это «пропущено».

Ссылки на старое ведут в
`/Users/nurmuhammad/Documents/workspace/IT/udevs/ucode-frontend/ucode_admin_frontend`,
референс — `src/views/views/` (предыдущие поколения не сравниваем).

## Auth

Референс: `src/views/Auth/` — `components/LoginFormDesign/`,
`components/RegisterFormPageDesign.jsx`, `components/RecoverPassword.jsx`,
`components/InviteForm.jsx`.

| Поведение | Где в старом | Статус | Чем закрыто |
|---|---|---|---|
| Вход логином и паролем | LoginFormDesign/LoginTab.jsx | есть | `LoginForm`, способ «Пароль» |
| Вход по коду в SMS | LoginFormDesign/PhoneLogin | есть | способ «Телефон»: `send-code-app` с `PHONE`, затем `type: "phone"` |
| Вход по коду на почту | LoginFormDesign/EmailAuth | есть | способ «Почта» — добавлен этим проходом; ручка его умеет (`session_v2.go`, случай `WithEmail`) |
| Вход через Google | ExternalAuth/GoogleAuthLogin.jsx | есть | `GoogleButton` + `type: "google"` |
| Выбор записи в connection'ах после входа | components/DynamicFields.jsx | есть | `ConnectionPicker`; варианты приходят прямо в ответе входа, отдельный `get-connection-options` не нужен |
| Выбор компании, проекта, окружения и client type диалогом | LoginFormDesign/LoginCompaniesList.jsx | выброшено | `/v3/multicompany/default-login` сам находит проект пользователя (`UserDefaultProject`), и спрашивать нечего |
| Восстановление пароля | components/RecoverPassword.jsx | есть | `RecoverForm`, все четыре шага ручек |
| Регистрация компании | components/RegisterFormPageDesign.jsx | есть | `RegisterForm` |
| Регистрация по приглашению | components/InviteForm.jsx | есть | `InviteForm` |
| Регистрация через Google | LoginFormDesign/index.jsx:158 (`register`) | выброшено | `company_service.go:78` декодирует токен без проверки подписи: чужой email забирается подделанным токеном. Вернём, когда появится проверка |
| Вход по ЭЦП (ESP) | LoginFormDesign/EspLogin/index.jsx | выброшено | это заглушка: `<Box>EspLogin</Box>` и ничего больше |
| Кнопка чата поддержки на экране входа | ExternalAuth/ChatwootLogin | выброшено | это виджет поддержки, а не способ входа; чужой скрипт на странице логина |
| OTP через Firebase | PhoneLogin/FireBaseOtp.jsx | выброшено | второй канал того же кода, со своим SDK и ключами в исходнике; ручка принимает и обычный `sms_id` |
| Флаги `new_router`, `newUi`, `newLayout`, `detailPage` в localStorage | LoginFormDesign/index.jsx:236 | выброшено | legacy-ветки не читаем вовсе — правило проекта |

## Actions (автоматизация)

Референс: `views/views/components/TableActions/TableActions.jsx`,
`views/views/components/ActionSettings/ActionSettings.jsx`,
`views/views/components/ActionsList/`.

| Поведение | Где в старом | Статус | Чем закрыто |
|---|---|---|---|
| Список действий таблицы, создание, правка, удаление | TableActions.jsx | есть | `features/table/api/actions.ts`, `TableActions` |
| Подпись по языкам данных, значок, адрес после успеха | ActionSettings.jsx:205 | есть | те же поля в нашей форме |
| Функция (`event_path`) | ActionSettings.jsx:220 | есть | выбор из функций проекта |
| Путь у функции-процесса (WORKFLOW) | ActionSettings.jsx:246 | есть | добавлено этим проходом: поле показывается, когда выбранная функция типа `WORKFLOW`; колонка `path` в `custom_event` была всегда, а задать её было нечем |
| Тип действия (HTTP / before / after) и метод | ActionSettings.jsx:257 | есть | те же списки значений |
| Перезапрос строк после успеха (`use_refresh`) | ActionSettings.jsx:276 | есть | список перезапрашивается сам |
| Выключенное действие | ActionSettings.jsx:285 | есть | пишем в колонку `disable`; старая форма писала `disabled`, и выключить действие через неё было нельзя |
| Запуск над отмеченными строками | — | есть | `useRunAction` → `/v1/invoke_function`; у старой админки в `views/` запуска не было вовсе, он остался в предыдущем поколении |
| Право роли на конкретное действие | ActionPermissionModal.jsx | есть | `isAllowed`: пустой `id` записи прав — это «никто не запрещал», а не запрет |
| Разбор ответа функции (`link`, `status: error`) | ActionButton.jsx:59 | выброшено | шлюз отвечает пустым телом (`function.go:557`) — ни ссылки, ни статуса; сообщение теперь «действие запущено», а не «выполнено». См. [backend-notes](backend-notes.md), «Функции» |
| `attributes.use_no_limit` | ActionSettings.jsx:281 | выброшено | в бэкенде такого параметра нет вовсе (`no_limit` не встречается ни в шлюзе, ни в сервисах): переключатель ничего не менял |
| Дополнительные параметры функции (`additional_parameters`) | ActionsList/TableRow.jsx | пропущено | значения сохраняем целыми при правке, но не показываем: у параметров три типа (TABLE, OBJECTID, HARDCODE) со связанными списками, и редактор без разбора их семантики запишет мусор в поле, которое сейчас хотя бы не теряется |
| Вызов микрофронтенда вместо функции | ActionButton.jsx:47 | пропущено | экран микрофронтенда есть, но действие открывает его модалкой поверх таблицы — это другое место монтирования; в проходе Constructor |

## Раскладка карточки

Референс: `views/views/components/HeaderFilter/components/LayoutComponent/`
и `views/Constructor/Tables/Form/Layout/` (`NewSection`, `NewSectionsBlock`,
`SummarySection`, `NewlayoutList`). Не референс — `views/Objects/LayoutSettings`.

| Поведение | Где в старом | Статус | Чем закрыто |
|---|---|---|---|
| Порядок полей в карточке перетаскиванием | NewSection.jsx (`applyDrag`) | есть | `model/layout`, `moveField` — с переносом В СЕКЦИЮ цели |
| Поле-заголовок карточки (`layout_heading`) | SettingsBlock.jsx | есть | `setHeading`, у мультиязычной таблицы — карта «язык → слаг» |
| Секции: завести, переименовать, удалить | NewSectionsBlock.jsx | есть | прямо в карточке; поля удалённой секции уходят в соседнюю, последнюю удалить нельзя |
| Имя секции переживает перезапрос | — | есть | наш обход: имя дублируется в `attributes.label`, потому что колонку `label` не возвращает ни один GET ([backend-notes](backend-notes.md)) |
| Поле спрятано из карточки (`field_hide_layout`) | FieldsBlock.jsx | есть | `hiddenFields`; колонкой таблицы то же поле остаётся |
| Права роли на поля (`field_permission`) | — | есть | `fieldRights` + `applyRights`: запрет строгий, разрешение по умолчанию |
| Вкладки связей: показать и убрать | RelationsBlock.jsx | есть | вкладка — это view пункта меню (`features/view/model/relation-tabs`), а не запись в раскладке: «+» в полосе вкладок и «⋯» у вкладки |
| Сводная секция (`summary_section`) | SummarySection.jsx | выброшено | сервер не отдаёт её ни под одним именем: `is_summary_section` нет в запросе секций, а `summary_fields` не собирают ни `GetAllV2`, ни `GetSingleLayout` — заполняет их одна мёртвая `GetAll` (`layout.go:763`), в которую grpc не заходит ([backend-notes](backend-notes.md)). В КАРТОЧКЕ старой админки её тоже нет — только константа в `defaultValues`, а в drawer'е строка закомментирована (`DrawerDetailPage/index.jsx:156`); живой редактор есть в конструкторе (`Constructor/Tables/Form/Layout/SummarySection.jsx`) — там же и вернётся к вопросу |
| Список раскладок таблицы, назначение раскладки пункту | NewlayoutList.jsx, LayoutsItem.jsx | пропущено | это экран конструктора таблицы, а не настройка карточки; вдобавок список не отдаёт `menu_id`, а `Layout().GetByID` шлюз наружу не выводит (зовёт только внутри `UpdateLayout`). В старой админке этот экран и сам сломан: `NewlayoutList.jsx:128` группирует по `layout?.menu_id`, которого в ответе нет. В проходе Constructor |
| Флаги раскладки: `is_default`, `is_modal`, `is_visible_section` | NewlayoutList.jsx:43–89 | пропущено | там же: они правятся из списка раскладок |
| Значок вкладки | LayoutTabs.jsx | выброшено | GET его не отдаёт, а PUT пишет колонку безусловно — значок стирался при любой правке раскладки |

## Шаблоны

Референс: `layouts/MainLayout/TemplateMenu/` (`index.jsx`, `TemplateTables.jsx`,
`TemplateSelection.jsx`), `services/templateService.js`,
`services/menuTemplateService.js`.

| Поведение | Где в старом | Статус | Чем закрыто |
|---|---|---|---|
| Список шаблонов проекта | TemplateSelection.jsx | есть | `TemplateDialog`, `GET /v1/template` |
| Развернуть шаблон в проект | TemplateSelection.jsx (`execute`) | есть | `useApplyTemplate`; после успеха меню перезапрашивается целиком |
| Сохранить папку шаблоном: имя, описание, таблицы | TemplateMenu/index.jsx (`onSubmit`) | есть | `TemplateCreateDialog`, `POST /v1/template` с `menu_id` папки |
| «Со строками» у таблицы | TemplateTables.jsx:56 | частично | флажок один на шаблон: при смешанном выборе шлюз отдаёт таблице без флага чужие строки (`template.go:106`, `345`) |
| Выбор отдельных таблиц ВНУТРИ шаблона при развёртывании | TemplateSelection.jsx | выброшено | `execute` читает из тела только `id` и `tables` целиком (`template.go:29`): выбор ничего не менял |
| Картинка шаблона | PhotoUpload.jsx, TemplateImage.jsx | выброшено | шаблон выбирают по имени в списке из нескольких строк; 400 строк редактора картинки на это не нужны. Поле `photo` в ответе читаем и показываем, если оно заполнено |
| Функции и микрофронтенды в шаблоне | FunctionsTable.jsx, MicroFunctions.jsx | выброшено | ключи шлюз кладёт в шаблон как есть, но собрать их было неоткуда: в старой форме оба списка всегда уходили пустыми |
| Вторая ручка `/v1/menu-template` | menuTemplateService.js | выброшено | параллельная ручка того же самого; старая админка выбирала между ними по состоянию всплывающего меню. Дерево пункта и так уезжает по `menu_id` |

## Микрофронтенды

Референс: `components/MicrofrontendComponent/index.jsx`,
`views/Microfrontend/index.jsx`, `layouts/MainLayout/MicrofrontendLinkModal.jsx`.
Решение о способе встраивания — [ADR-0005](adr/0005-microfrontend-embedding.md).

| Поведение | Где в старом | Статус | Чем закрыто |
|---|---|---|---|
| Пункт меню открывает чужое приложение | MenuSwitchCase.jsx:20 | есть | `features/microfrontend`, ветка в маршруте пункта |
| Адрес сборки: `https://<url>/assets/remoteEntry.js` | Microfrontend/index.jsx:30 | есть | `entryUrl`; схему не удваиваем, хвостовую косую снимаем |
| Загрузка модуля `./Page` федерацией | MicrofrontendComponent/index.jsx:23 | есть | `@module-federation/runtime`, `type: "module"`; рантайм грузится по требованию |
| Перемонтирование при каждом заходе (`activationKey`) | MicrofrontendComponent/index.jsx:31 | есть | ключ из id и адреса |
| Пропсы: окружение, `i18n`, оси http | MicrofrontendComponent/index.jsx:59 | есть | те же имена; вместо axios 0.26 — наши `http` и `httpAuth`. Форма ответа отличается: наш перехватчик снимает конверт |
| Заведение пункта: выбор приложения и параметры | MicrofrontendLinkModal.jsx | есть | «+» → «Создать микрофронтенд»: список приложений проекта и редактор пар «ключ — значение» |
| Смена приложения у готового пункта | MicrofrontendLinkModal.jsx (правка) | выброшено | `PUT /v3/menus` колонку `microfrontend_id` не пишет вовсе (`menu.go:944`) — список в форме правки был бы переключателем, который ничего не переключает |
| `attributes.params` в query-строке адреса | MenuSwitchCase.jsx:25 | частично | передаём их пропсом `params`, а не через адрес: у нас пункт живёт по `/m/$menuId`, и своей строки запроса у ремоута нет. Ремоут, читающий `location.search`, их не увидит |
| Общие `react`, `react-dom`, `react-router-dom` | vite.config.js:25 | выброшено | ремоуты собраны против React 18, у нас 19; общий синглтон отдал бы им нашу версию. У каждого свой React — штатный запасной путь федерации |
| Отдельная страница-песочница для ремоута | views/MicrofrontendPlayground | выброшено | поле ввода для произвольного `remoteEntry.js` — инструмент отладки, а не экран админки |
| Микрофронтенд на экране входа | layouts/AuthLayout/LoginMicrofrontend.jsx | пропущено | другое место монтирования и другой набор пропсов (`loginAction`); вместе с настройками проекта |
| Экран конструктора: список, версии, промоут, откат | SettingsPopup/modules/MicroFrontend | пропущено | это конструктор, а не пункт меню; в проходе Constructor |

## Настройки меню

Референс: `layouts/MainLayout/` (`TableCreateModal`, `FolderCreateModal`,
`LinkTableModal`, `MenuSettingModal`), `services/menuSettingService.js`.

| Поведение | Где в старом | Статус | Чем закрыто |
|---|---|---|---|
| Имя пункта по языкам, значок | FolderCreateModal.jsx:141 | есть | `MenuFormDialog` |
| Слаг таблицы при создании | TableCreateModal.jsx:304 | есть | шаг с ключом; таблица заводится ручкой `/v1/table` |
| Адрес у пункта-ссылки | LinkTableModal.jsx:177 | есть | поле адреса, проверка протокола (`safeHref`) |
| Пункт-ссылка на СУЩЕСТВУЮЩУЮ таблицу (`type: LINK` + `table_id`) | LinkTableModal.jsx:53 | есть | добавлено этим проходом: такой пункт открывается таблицей, а не объяснением «экрана нет» (`showsTable`) |
| Настройки самой таблицы: кэш, мягкое удаление, вход, сортировка | TableCreateModal.jsx:326 | есть | страница «Настройки таблицы» в панели view |
| Вид сайдбара: тип меню, размер пунктов, шаблон | MenuSettingModal.jsx, `/menu-settings` | выброшено | у сайдбара один вид, заданный дизайн-системой |
| Права роли на пункт (`menu_permission`) | SettingsPopup/PermissionsRoleDetail | пропущено | читаем на каждом пункте, но не настраиваем — в проходе Settings |
| Системные пункты: избранное, аналитика, сводная, отчёты | STATIC_MENU_IDS | частично | прячем только настройки, файлы и пользователей — у них свои экраны; остальные видны и открываются объяснением, пока экранов нет ([PLAN-v3](PLAN-v3.md), §1) |

## Sidebar

Референс: `src/components/LayoutSidebar/` (`index.jsx`,
`SidebarRecursiveBlock/RecursiveBlockComponent.jsx`, `MenuButtons.jsx`),
`src/hooks/useSidebarElements/`.

| Поведение | Где в старом | Статус | Чем закрыто |
|---|---|---|---|
| Дерево пунктов меню с вложенными папками | SidebarRecursiveBlock/RecursiveBlockComponent.jsx | есть | `MenuLevel`, рекурсивно |
| Пункт скрыт без права чтения | hooks/useSidebarElements/index.jsx:19 | есть | `toNodes` отбрасывает `can.read === false` |
| Перетаскивание пунктов и папок | index.jsx (`react-smooth-dnd`, `applyDrag`) | есть | `model/reorder.ts` — чистая функция с тестами, порядок пишется `PUT /v3/menus` |
| Раскрытые папки помнятся между заходами | store/menus (menuAccordionActions) | есть | `expandedMenus` в `ui-store`, забывается вместе с удалённым пунктом |
| Свой значок пункта (SVG по ссылке) | LayoutSidebar/MenuIcon.jsx | есть | `shared/ui/dynamic-icon`, запасной значок по типу |
| Создание таблицы, папки и ссылки | index.jsx (TableCreateModal, FolderCreateModal, LinkTableModal) | есть | `AddMenuButton` и меню строки |
| Переименование и удаление пункта | RecursiveBlockComponent.jsx:589 | есть | `MenuRowActions` |
| Сворачивание сайдбара | index.jsx (KeyboardDoubleArrow…) | есть | `CollapseButton` плюс подглядывание по наведению |
| Переключение проекта и окружения | index.jsx (useCompanyListQuery, useEnvironmentListQuery) | есть | `WorkspaceSwitcher` в шапке |
| Выход, профиль, язык интерфейса | index.jsx (Logout, TranslateIcon) | есть | меню шапки: профиль, язык, тема, выход |
| Ширина сайдбара тянется мышью | — | есть | наше добавление, у старого ширина постоянная |
| Создание wiki, wiki-папки, микрофронтенда, minio-папки, сайта | RecursiveBlockComponent.jsx:183–218 | частично | заводим то, у чего есть экран: папка файлов, встроенная страница (флажок у ссылки) и микрофронтенд. Wiki не заводим — экрана за ней нет и не будет, пока нет ручек. Пункт, ведущий на заглушку, заводить незачем |
| «Настройки пункта» (вид сайдбара: размер, стиль) | layouts/MainLayout/MenuSettingModal.jsx | выброшено | у сайдбара один вид, заданный дизайн-системой; настройка меняла размеры и стиль пунктов |
| «Сделать шаблоном» из папки | layouts/MainLayout/TemplateMenu | есть | `TemplateCreateDialog` в «⋯» папки — см. раздел «Шаблоны». Кнопка была убрана, пока за ней ничего не стояло |
| Кнопки разделов: API, функции, запросы, сценарии, документы, Minio, права | LayoutSidebar/MenuButtons.jsx | пропущено | это отдельные экраны конструктора, не сайдбар; их черёд в обходе — Constructor и Settings |
| Приглашение пользователя из сайдбара | components/InviteModal | пропущено | принять приглашение мы умеем (`InviteForm`), отправить — нет; место ему в настройках проекта |
| Создание организации | LayoutSidebar/AddOrganization.jsx | пропущено | вместе с экраном компаний |
| Баннер подписки | utils/subscriptionWarning | выброшено | биллинга в проекте нет |

## TIMELINE

Референс: `src/views/views/modules/Timeline/`.

| Поведение | Где в старом | Статус | Чем закрыто |
|---|---|---|---|
| Полоса от `calendar_from_slug` до `calendar_to_slug` | TimeLineDayDataBlockItem.jsx:176 | есть | `model/timeline.ts`, `timelineBar` |
| Лента месяцев без конца, подгрузка у краёв | hooks/useDateLineProps.jsx:66 | есть | `onLoadPast`/`onLoadFuture`, ±1 месяц за шаг вместо ±3 |
| Поправка прокрутки при росте ленты влево | useDateLineProps.jsx:101 | есть | `anchored` в `Timeline.tsx` |
| Масштаб оси: дни, недели, месяцы | TimeLineBlock.jsx:217 | есть | Те же три. «Недели» у старого отличались от «дней» только подписью в шапке (ширина колонки та же, 60px) — у нас колонка вдобавок уже, иначе средний масштаб ничего не даёт |
| Прокрутка к сегодня на открытии и смене масштаба | TimeLineDayBlock.jsx:93 | есть | `focus` + кнопка «Сегодня»; при смене масштаба возвращаемся не на сегодня, а на день у левого края |
| Подпись месяца едет с прокруткой | TimeLineDatesRow.jsx:159 | есть | `sticky` подпись в шапке |
| Выходные другим фоном (кроме масштаба месяца) | TimeLineDayBlock.jsx:89 | есть | слой колонок в `Timeline.tsx` |
| Отметка «сегодня» в шапке | TimeLineDayBlock.jsx:86 | есть | кружок на числе + подложка колонки |
| Подсветка дней полосы в шапке на наведении и переносе | TimeLineDayDataBlockItem.jsx:312 (`setFocusedDays`) | есть | `focused` в `Timeline.tsx` |
| Подсветка строки на наведении | TimelineBlockProvider (`hoveredRowId`) | есть | `group/row` в разметке |
| Перенос полосы по дням, PUT обеих дат | TimeLineDayDataBlockItem.jsx:230 | есть | `drag.kind === "move"` |
| Растягивание за левый и правый край | Moveable `renderDirections: ["w","e"]` | есть | ручки `EventChip`, `drag.kind === "resize"` |
| Прилипание к суткам при переносе | `throttleDrag` | есть | день считается из `dayAt`, дробных дней нет |
| Щелчок по полосе открывает запись | TimeLineDayDataBlockItem.jsx:70 | есть | `onOpenRow` |
| Свой адрес перехода (`attributes.navigate`) | Timeline.jsx:110 | есть | `openRowUrl` в маршруте |
| Список записей слева сворачивается | components/Sidebar + SidebarButton | есть | кнопка сворачивания; у нас список есть всегда, у старого — только при группировке |
| Записи без дат видны и ставятся на ось | TimeLineBlock.jsx:161, TimeLineDataRecursiveRow.jsx:78 | есть | раздел «Без дат» + протяжка. Старый ставил `сегодня…+5 дней` по щелчку — у нас даты берутся из протяжки |
| Поля, показанные на полосе | ColumnsVisibility/useColumnsVisibilityProps.jsx:84 (`attributes.visible_field`) | есть | пишем `columns`, а `visible_field` читаем запасным вариантом ради старых view (`resolveColumns`) |
| Подсказка с датами при протяжке | components/TimelineRowNewDateLine | есть | подпись у выделения и у призрака |
| Настройка полей дат | HeaderFilter/components/TimelineSettings | есть | тот же `CalendarFields`, что у календаря |
| Одинаковые поля начала и конца | TimeLineBlock.jsx:61 (красное окно, экран пуст) | выброшено | такой выбор просто не предлагается в списке |
| Однодневная задача | TimeLineDayDataBlockItem.jsx:387 (`differenceInDays > 0` — полоса скрыта) | выброшено | у нас минимум одна колонка: скрывать запись, у которой начало равно концу, нечестно |
| Группировка: разделы слева, свёртка, вложенность | components/TimelineRecursiveRow, Sidebar | есть | Столько уровней, сколько полей в `attributes.group_by_columns`, той же разбивкой, что у таблицы (`model/group`); отступ по уровню |
| «Expand all / Collapse all» над группами | Sidebar.jsx:47 | выброшено | Появлялась только при двух уровнях и более; на одном сворачивать нечего оптом |
| Заголовок списка слева («Columns») | Sidebar.jsx:44 | есть | «Записи» |
| «＋ New» внизу списка | Sidebar.jsx:100 | есть | пустая строка внизу: по ней протягивают даты новой записи |
| Свой запрос `gte`/`lte` в теле | Timeline.jsx:149 | выброшено | ключи не разбираются бэкендом (`build_query.go:256`, `applyFilters`): отбора по диапазону у старого экрана нет вовсе, приезжает вся таблица. У нас — `{поле: {$gte,$lte}}` |
| `view_type: "TIMELINE"` в теле | Timeline.jsx:148 | выброшено | ручка берёт из тела только ключи-слаги полей (`object_builder.go:1249`) |
| `builder_service_view_id` (серверная группировка) | Timeline.jsx:152 | выброшено | ветка игнорирует фильтры, права и страницу — [backend-notes](backend-notes.md), «Группировка» |
| Всплывающая подсказка на клетке шапки | TimeLineDayBlock.jsx:152 | есть | `title` с полной датой; день недели вдобавок написан в самой шапке |
| Число в узком масштабе — не у каждого дня | TimeLineMonth.jsx:88 | есть | понедельник, сегодня и края подсвеченного отрезка |
| Подсказки по наведению (день оси, полоса, кнопки) | TimeLineDayBlock.jsx:152 (MUI Popover) | есть | свой `shared/ui/tooltip`, а не браузерный `title` |
| Зум-множитель `zoomPosition` | TimeLineBlock.jsx:234 | выброшено | у старого он выводится из масштаба и другого значения не принимает — это не настройка, а константа |

## Constructor: таблица, поля, связи

Где смотреть: `views/Constructor/Tables/Form/NewRouterTable.jsx` — шесть
вкладок («Details», «Layouts», «Fields», «Relations», «Actions», «Custom
errors»), плюс `TablesList.jsx` и `ImportModal.jsx`. Не референс:
`OldRouterTableSettings.jsx`, `Layout-backup/`, `Actions-backup/`,
`views/Objects/`.

Отдельного экрана конструктора у нас нет намеренно: настройки таблицы
и полей открываются там, где на них смотрят, — в панели view
(`ViewOptions` → `TableSettings`) и в панели поля у заголовка колонки
(`FieldEditor`). Экран, ради которого уходят со своих данных, — это
и есть причина, по которой в старой админке настройки разошлись
с тем, что видно.

**Пройдено этим проходом:** все шесть вкладок, список таблиц и импорт.
Настройки связи сверены по колонкам таблицы `relation`
(`000001_init_tables.up.sql:104`), а не по форме старой админки: форма
показывает и то, чего ни одна ручка не пишет.

| Что | Где в старом | Статус | Как у нас |
|---|---|---|---|
| Имя таблицы по языкам данных | MainInfo.jsx:117 | есть | `LanguageInput` в `TableSettings` |
| Слаг таблицы | MainInfo.jsx:132 | выброшено | `UPDATE "table"` колонки `slug` не пишет (`table.go:887`): поле принимало новый слаг и отвечало «сохранено», не меняя ничего. Показан, но не правится |
| Кэш, мягкое удаление, ручная сортировка | MainInfo.jsx:157–172 | есть | те же три флажка |
| Таблица входа + способы входа | MainInfo.jsx:148, 336 | есть | включение ждёт первого способа: ручка отказывает пустому `login_strategy` (`table.go:970`) |
| Шесть списков `auth_info` (тип клиента, роль, логин, пароль, email, телефон) | MainInfo.jsx:197–308 | выброшено | бэкенд собирает `auth_info` сам и перезаписывает присланное (`table.go:1007`) |
| Поле: тип, слаг, подпись по языкам, обязательность, уникальность, только чтение, проверка и текст отказа, значение по умолчанию, автозаполнение, мультиязычность | FieldSettings.jsx:463–845 | есть | `FieldEditor` + `model/field-draft`; правки копятся в черновике и уезжают одним PUT |
| Атрибуты по типам: приставка и разрядность номера, ключ карт и точка, варианты списка, формула, агрегат с отбором | Fields/Attributes/* | есть | `TypeSettings`, `FormulaSettings`, `AutofillSettings`, `ButtonSettings` |
| Тип `MONEY` | fieldTypes.js:129 | выброшено | мёртвая зона: ввести значение нечем (редактора нет ни в одном поколении), а ячейка не рисуется вовсе — `CellElementGenerator.jsx:278` возвращает разметку ТОЛЬКО когда сумма пуста. Настроек валюты нет: в базе это `TEXT[]` (`pkg/helper/convert.go:46`). Деньги сегодня — `FLOAT` рядом со списком валют |
| Тип `PERSON` | fieldTypes.js:286 | выброшено | это не тип, а ярлык: `CreateField` при `PERSON` заводит Many2One на системную таблицу `person` и выходит, не вставляя поле (`field.go:70–102`), а сама колонка получает тип `LOOKUP`. Наша форма связи делает ровно это и явно |
| Типы, закомментированные в старом списке (`PROGRAMMING_LANGUAGE`, `BARCODE`, `CODABAR`, `SCAN_BARCODE`, `DENTIST`, `AUTOFILL`) | fieldTypes.js:101–163 | выброшено | заводить их нельзя и в старой админке. Уже заведённые читаются: `model/cell-kind` знает их все, кроме `DENTIST` — тот показывается текстом |
| Скрыть поле по значению другого (`attributes.hide_path`) | FieldSettings.jsx:875–949 | пропущено | настройка есть в обеих формах старой админки, а ПРИМЕНЯЕТ её только `views/Objects/NewMainInfo.jsx:45` — предыдущее поколение. В нынешней карточке (`views/views/`) её не читает никто: поле настраивается и не прячется |
| Вкладка «Custom errors» | CustomErrors/index.jsx | заблокировано | `/v2/collection/{slug}/error_messages` в шлюзе нет: ни маршрута в `api/api.go`, ни обработчика — только proto и сгенерированный код. Старая админка зовёт несуществующую ручку |
| Вкладка «Layouts» | Form/Layout/ | см. «Раскладка карточки» | секции и поля правятся в карточке; список раскладок и назначение пункту заблокированы ответом сервера |
| Вкладка «Actions» | Form/Actions/ | есть | см. раздел «Actions» |
| Связь: целевая таблица и поля показа | Relations/RelationCreateForm.jsx | есть | панель поля, тип «Связь»; направление одно — Many2One (`model/relation-draft`) |
| Автофильтр связи (`auto_filters`) | Relations/AutoFiltersBlock.jsx | есть | пары «поле этой записи → поле чужой таблицы» задаются в форме связи и применяются при выборе строки (`model/relation-draft` + `autoFilterValues`). Раньше настройка не читалась вовсе и приезжала вся чужая таблица |
| Правится ли связь из таблицы (`editable`) | RelationSettings.jsx | выброшено | колонку `relation.editable` в нынешнем поколении не читает НИКТО: живёт только `view.attributes.table_editable` (`TableDataForm.jsx:76`), а это настройка view, и она у нас есть |
| Подставлять текущего пользователя (`is_user_id_default`, `object_id_from_jwt`) | Relations/DefaultValueBlock.jsx | пропущено | настройка живая: в старой админке она задаёт значение по умолчанию у НОВОЙ записи (`FormElementGenerator.jsx:126`) и подмешивает свою строку в список выбора (`RelationField.jsx:128`). Колонка приезжает в списке связей (`relation.go:2805`), так что дело за нашей стороной: `blankItem` знает только поля, а тут нужны связь и id текущего пользователя (`session.getUserId`) |
| Каскадные списки (`cascadings`, `cascading_tree_*`) | CascadingRelationSettings.jsx | пропущено | колонки есть; это отдельная фича размером с автофильтр, но с деревом |
| Many2Dynamic (`dynamic_tables`) | DynamicRelationsBlock.jsx | пропущено | связь на таблицу, выбираемую строкой; из таблицы её не завести — см. `model/relation-draft` |
| Список таблиц проекта, создание, удаление | TablesList.jsx | есть | таблицы заводятся из сайдбара («+» → «Таблица»), удаляются в `TableSettings`; отдельного списка нет — им служит само дерево пунктов |
| Импорт таблиц из другого приложения | ImportModal.jsx | заблокировано | всё это про сущность «приложение»: `applicationService` зовёт `/v1/app`, а такого маршрута в шлюзе нет вовсе. Вместе с ним мертвы `is_own_table` и `is_visible`, по которым `TablesList.jsx:88` выбирает, удалять таблицу или всего лишь отвязать её от приложения |

## Печатные формы («Docs»)

Где смотреть: пункт «Docs» в поповере настроек view
(`views/views/components/HeaderFilter/components/ViewOptions/ViewOptions.jsx:373`)
ведёт на `/{menuId}/templates` — экран `views/DocumentTemplates/`
(~1000 строк). Не референс: `Documents/Components/Template/` — это
второе, HTML-поколение тех же шаблонов.

Суть фичи: к таблице прикладывают файл .docx с переменными вида
`{слаг}`, а потом печатают им КОНКРЕТНУЮ запись — бэкенд подставляет
значения строки и отдаёт PDF (`POST /v2/docx-template/convert/pdf`,
шлюз `docx_template.go:809`).

| Что | Где в старом | Статус | Как у нас |
|---|---|---|---|
| Список шаблонов таблицы | DocumentTemplates/index.jsx:56 | есть | `features/docs`, панель в настройках view рядом с настройками таблицы: шаблоны у таблицы общие, во всех её view одни и те же |
| Завести шаблон | — (заводился редактором) | есть | загрузка готового .docx тем же загрузчиком, что и файл в ячейку; имя берётся из имени файла |
| Удалить шаблон | index.jsx:237 | есть | с подтверждением; сам файл в хранилище остаётся — его удаление ручкой не предусмотрено |
| Подсказка по переменным | components/Variables/index.jsx:40 | есть | список полей, щелчок кладёт `{слаг}` в буфер. Читаем из своей схемы: ручка `/v2/docx-template/fields/list` отдаёт те же поля и всегда пустой список связей (`docx.go:514`, ключ там назван «relations:» — с двоеточием) |
| Печать записи в PDF | index.jsx:101 (`onPDFDownloadClick`) | есть, и ближе к данным | кнопка в карточке записи: строка уже открыта, и заполнять форму заново не нужно. В старом это отдельный экран, куда запись передаётся параметром `?id=` |
| Встроенный редактор .docx | components/Editor/index.jsx | выброшено | это ONLYOFFICE Document Server — отдельная служба, которой браузер сам подписывает JWT секретом `my_jwt_secret`, лежащим в исходнике старой админки. Шаблон делают в Word и загружают готовым |
| Просмотрщик PDF внутри админки | components/Viewer/index.jsx | выброшено | `@react-pdf-viewer` + свой воркер pdf.js ради предпросмотра; PDF открывается вкладкой браузера, у которого просмотрщик уже есть |
| Форма записи на экране шаблонов | components/ObjectForm/ | выброшено | нужна была потому, что печать жила отдельно от данных: экран заново собирал секции раскладки и значения строки. Печать из карточки делает это лишним |
| HTML-шаблоны (`/v1/html-template`) | services/documentTemplateService.js | пропущено | второе поколение той же фичи, живёт параллельно docx; кнопка «Docs» ведёт не сюда. Вернёмся, если окажется, что проекты пользуются им, а не docx |
| Экспорт HTML/PDF из HTML-шаблона | documentTemplateService.js:9 | заблокировано | зовёт `/v2/utils/export/{slug}/html-to-pdf`, а такого маршрута в шлюзе нет вовсе; работающие конвертеры лежат на `/v1/html-to-pdf` и `/v1/template-to-html` |
