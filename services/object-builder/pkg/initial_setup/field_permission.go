package initialsetup

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateDefaultFieldPermission(conn *pgxpool.Pool, roleId string) error {
	query := `INSERT INTO field_permission("guid", "role_id", "label", "table_slug", "field_id", "edit_permission", "view_permission") 
    VALUES 
    ('802b4f04-9d85-45cc-9016-3d8b1ca5bfa2', $1, 'Название', 'project', '37137f5f-ef9b-4710-a6df-fb920750fdfb', true, true),
    ('a2c47645-c470-4add-8cf4-fe771ebb4a10', $1, 'Домен проекта', 'project', 'dfbf6a89-9c78-4922-9a00-0e1555c23ece', true, true),
    ('4fc8f5d3-d7d7-4290-9017-c8d47e6d1644', $1, 'ID', 'project', '8265459c-ab41-45b5-a79d-cbfa299ddaa7', true, true),
    ('f59f17ae-8873-42ca-9854-e2122b7b2ff1', $1, 'Субдомен платформы', 'client_platform', '948500db-538e-412b-ba36-09f5e9f0eccc', true, true),
    ('adc920b0-8501-4186-a094-b2f7117b92d0', $1, 'Название платформы', 'client_platform', 'c818bc89-c2e9-4181-9db4-06fdf837d6e2', true, true),
    ('3ca58e92-5ae0-4ef0-9e45-c072cc228377', $1, 'IT''S RELATION', 'client_platform', 'd95156ba-d443-4c95-8383-c122747330c5', true, true),
    ('c27601cf-a256-4009-9006-3b20a14c976d', $1, 'ID', 'client_platform', '6c812f3d-1aae-4b9e-8c28-55019ede57f8', true, true),
    ('d553f68f-ebe4-4a0c-96fc-40123ba3a9ce', $1, 'IT''S RELATION', 'client_platform', 'f7220ec5-d9cb-485b-9652-f3429132375d', true, true),
    ('86ee9f3f-7066-4ab5-903a-57519db4d34f', $1, 'Подтверждено', 'client_type', 'd99ac785-1d1a-49d8-af23-4d92774d15b6', true, true),
    ('861e6969-5881-4438-88ba-4e021b1d12f0', $1, 'ID', 'client_type', '5bcd3857-9f9e-4ab9-97da-52dbdcb3e5d7', true, true),
    ('26294b4f-9559-4d85-82ee-8828d6ac7d37', $1, 'IT''S RELATION', 'client_type', 'faa90368-d201-4322-82b7-e370f788d248', true, true),
    ('442f5e92-5893-4707-920d-afe571d54d3c', $1, 'Самовосстановление', 'client_type', 'd37e08d6-f7d0-441e-b7af-6034e5c2a77f', true, true),
    ('1f4c2339-2613-42e3-85e9-c438fe796b5f', $1, 'IT''S RELATION', 'client_type', '4eb81779-7529-420f-991f-a194e2010563', true, true),
    ('1588aeaa-9354-4b0e-91c9-96a479ed2d9b', $1, 'Название типа', 'client_type', '04d0889a-b9ba-4f5c-8473-c8447aab350d', true, true),
    ('39ddc721-2844-4d50-9720-0d4d019b6516', $1, 'Саморегистрация', 'client_type', '763a0625-59d7-4fd1-ad4b-7ef303c3aadf', true, true),
    ('efa21a0f-3bbe-443b-aa57-6f43a5b33239', $1, 'ID', 'role', '3bb6863b-5024-4bfb-9fa0-6ed5bf8d2406', true, true),
    ('7db28cca-d856-45cc-9ecd-332ed407a643', $1, 'Название роли', 'role', 'c12adfef-2991-4c6a-9dff-b4ab8810f0df', true, true),
    ('8d4dd8ff-22ba-49f6-b751-75a8721d6de8', $1, 'IT''S RELATION', 'role', 'cb677e25-ddb3-4a64-a0cd-5aa6653417ed', true, true),
    ('7a7241df-5abd-4ffe-b396-017df772950c', $1, 'IT''S RELATION', 'role', '110055ac-75ab-4c1f-ae35-67098d1816a5', true, true),
    ('172f3b2d-c02a-4501-9cde-5fa9fd310cc0', $1, 'IT''S RELATION', 'role', '123cd75b-2da5-458f-8020-8176a18b54ce', true, true),
    ('a5035729-d6e6-4040-b679-f510ec12c16f', $1, 'Актив', 'user', '645d85b8-67e0-4594-96c7-540b63d6b018', true, true),
    ('aeab778f-81a8-4335-90a8-3d07b18678a1', $1, 'IT''S RELATION', 'user', '5ca9db39-f165-4877-a191-57b5e8fedaf5', true, true),
    ('2c5b1ec3-df59-48cc-b3c2-52a3a7458212', $1, 'IT''S RELATION', 'user', 'd84b1431-f7ae-43c5-b2e1-83f82ec1f979', true, true),
    ('bcfc6e8c-cedd-44f1-a388-566b7e9de372', $1, 'ID', 'user', '2a77fd05-5278-4188-ba34-a7b8d13e2e51', true, true),
    ('bf8d0e71-8538-4f66-b0ad-185a5147be08', $1, 'IT''S RELATION', 'user', 'be11f4ac-1f91-4e04-872d-31cef96954e9', true, true),
    ('b8b5aa3d-b912-4f5a-93e6-bb1dd650bf4b', $1, 'IT''S RELATION', 'user', 'bd5f353e-52d6-4b07-946c-678534a624ec', true, true),
    ('6f11bbc4-3ffd-43d2-b335-1bab3769eee2', $1, 'Table Slug', 'connection', '9d53673d-4df3-4679-91be-8a787bdff724', true, true),
    ('480d4dc7-ae45-40f9-813b-f040590d1ec1', $1, 'View Slug', 'connection', 'a9767595-8863-414e-9220-f6499def0276', true, true),
    ('7faa5a72-2014-4240-b50a-520a22ecf34f', $1, 'Тип', 'connection', '71a33f28-002e-42a9-95fe-934a1f04b789', true, true),
    ('1e564b97-9f35-4a47-9c21-53c2afb0e8e1', $1, 'Icon', 'connection', '546320ae-8d9f-43cb-afde-3df5701e4b49', true, true),
    ('35130880-0c81-4ab6-83fd-377fa67d15bf', $1, 'View label', 'connection', 'b73c268c-9b91-47e4-9cb8-4f1d4ad14605', true, true),
    ('1a4e0e5a-54ea-4c8f-beaa-145a7a8d5821', $1, 'ID', 'connection', 'fbcf919a-25b8-486b-a110-342c8c47385f', true, true),
    ('3bb378f9-3383-4247-9151-c6343a4a0f65', $1, 'Название', 'connection', 'c71928df-22d1-408c-8d63-7784cbec4a1d', true, true),
    ('b5be8f59-88c5-4f9d-90e8-eb4a09019442', $1, 'IT''S RELATION', 'connection', 'f6962da4-bc72-4236-ac7b-18589c2fc029', true, true),
    ('b5a635d5-a6fc-47dd-a0f1-dc471a00b816', $1, 'Написать', 'record_permission', '1e71486b-1438-4170-8883-50b505b8bb14', true, true),
    ('dde022cc-f22f-4b92-8fee-16c347ef14d8', $1, 'ID', 'record_permission', '4d1bb99b-d2f0-4ac7-8c50-2f5dd7932755', true, true),
    ('70c8d94d-59bc-44a2-b7bf-113860ac8bbd', $1, 'Удаление', 'record_permission', 'd95c1242-63ab-45c1-bd23-86f23ee72971', true, true),
    ('dcbb9777-af4b-458f-b0b8-80c658f66684', $1, 'IT''S RELATION', 'record_permission', '8e748044-1b00-485c-b445-63e44380a6b1', true, true),
    ('c10b0951-f26f-4a39-94d2-aedc2d1c100b', $1, 'Изменение', 'record_permission', 'f932bf71-9049-462b-9342-8347bca7598d', true, true),
    ('b3b1e71e-0069-47f6-aca7-f4a9a4ce83bc', $1, 'Чтение', 'record_permission', '27355d70-1c11-4fb9-9192-a1fffd93d9db', true, true),
    ('d0a519b7-2cd5-4507-9dc1-1020fe564b24', $1, 'Название таблица', 'record_permission', '9bdbb8eb-334b-4515-87dc-20abd0da129a', true, true),
    ('884565d6-8940-44d2-bd6d-6e28eaa4793a', $1, 'IT''S RELATION', 'test_login', 'd5fda673-95b2-492a-97c2-afd0466f1e32', true, true),
    ('f54c18ca-c5c7-4b95-a196-2afd5db7d341', $1, 'Login label', 'test_login', '5591515f-8212-4bd5-b13b-fffd9751e9ce', true, true),
    ('abda5537-cb1f-41c8-bec5-f150dd0ef692', $1, 'Password view', 'test_login', '35ddf13d-d724-42ab-a1bb-f3961c7db9d6', true, true),
    ('c8ddae0a-c3dc-44ba-aab5-adc67e2a7aef', $1, 'Table Slug', 'test_login', 'f5486957-e804-4050-a3c5-cfdcdaca0a16', true, true),
    ('68ea4128-efbb-4359-933d-704bf92a8832', $1, 'Login view', 'test_login', 'fbc9b3e9-0507-48f5-9772-d42febfb4d30', true, true),
    ('65c2a192-eb3a-484d-85dd-ab41968c4485', $1, 'ID', 'test_login', '14e3ed9a-384e-45ac-897f-0fb4174adfaf', true, true),
    ('557919a0-691f-43cc-9d00-9230aa692a32', $1, 'Login strategy', 'test_login', 'd02ddb83-ad98-47f5-bc0a-6ee7586d9a8e', true, true),
    ('e09c9140-038f-48ce-a89e-78af27c9b911', $1, 'Password label', 'test_login', '276c0e0c-2b79-472a-bcdf-ac0eed115ebe', true, true),
    ('192bde8b-e22b-4722-a5e1-800bd3b6257c', $1, 'Ид обьеткта', 'test_login', '7ab42774-6ca9-4e10-a71b-77871009e0a2', true, true),
    ('eb9c8c5e-aa13-4c9b-9490-15afc47fd47c', $1, 'Название таблица', 'automatic_filter', '8368fc76-0e80-409c-b64e-2275304411d8', true, true),
    ('bc7f9be9-127b-467e-a0bb-2de6769dd0c6', $1, 'ID', 'automatic_filter', '2ca6eec7-faea-4afd-a75f-980c18164f3c', true, true),
    ('068e0b08-b834-4829-ab07-46874a6a03ee', $1, 'IT''S RELATION', 'automatic_filter', 'a1ece772-a8e0-41ae-8060-e1f667d0d96e', true, true),
    ('f70cbe72-42ef-4ac6-8a76-60f9f4f755a0', $1, 'Полья объекты', 'automatic_filter', '957ffe32-714a-41d2-9bd8-e6b6b30fef67', true, true),
    ('e13bb70f-fed0-4fb7-a871-760273edf662', $1, 'Пользавательские полья', 'automatic_filter', '6d5d18cd-255d-49fd-a08e-5a6b0f1b093f', true, true),
    ('2475592e-3cff-44bc-adee-dda134d420ab', $1, 'ID', 'field_permission', '6040f51f-7b41-4e7a-87b1-b48286a00bea', true, true),
    ('d02124b2-5f2f-47cf-88d2-1f865f92baf4', $1, 'Название таблица', 'field_permission', '27634c7a-1de8-4d54-9f57-5ff7c77d0af9', true, true),
    ('3198134c-b258-4e69-9394-afa3b35423a7', $1, 'Ид поля', 'field_permission', '7587ed1d-a8b9-4aa8-b845-56dbb9333e25', true, true),
    ('b7522fe1-1975-4497-b015-fd8e3f8b4d12', $1, 'Изменить разрешение', 'field_permission', '9ae553a2-edca-41f7-ba8a-557dc24cb74a', true, true),
    ('d1bcee2f-9a78-4355-b291-0fad2b42dcc9', $1, 'IT''S RELATION', 'field_permission', '1267fb0d-2788-4171-ab69-b9573d3974a2', true, true),
    ('a07ed267-5883-40e5-8304-374542f5c9d1', $1, 'Разрешение на просмотр', 'field_permission', '00787831-04b4-4a08-b74d-14f80a219b86', true, true),
    ('a7f6e5fb-421b-4fa8-b4b6-3f3a2c34502b', $1, 'Название поля', 'field_permission', '285ceb40-6267-4f5e-9327-f75fe79e8bfe', true, true),
    ('696c3230-3b16-4a44-bf3e-4e2f271ad2a0', $1, 'Есть условия', 'record_permission', '5f099f9f-8217-4790-a8ee-954ec165b8d8', true, true),
    ('35b0de7c-a522-4ec0-bf92-650d41a09a42', $1, 'Ид действия', 'action_permission', '1e39a65d-9709-4c5a-99e4-dde67191d95a', true, true),
    ('5fa8d656-b21f-4b65-855e-a00ed1f55ae0', $1, 'Название таблица', 'action_permission', '34abee63-37ad-48c1-95d2-f4a032c373a1', true, true),
    ('993e6d77-35a4-48b4-b80a-a253731b9853', $1, 'Разрешение', 'action_permission', 'b84f052c-c407-48c5-a4bf-6bd54869fbd7', true, true),
    ('3eb47ce0-7635-4aa8-a216-783bb9722526', $1, 'ID', 'action_permission', 'cf504dda-a34a-4422-843b-afaa32efbe49', true, true),
    ('e4f0c2e7-7958-4a5a-9270-41f91c591cc6', $1, 'Название таблица', 'view_relation_permission', 'd8127cf2-2d60-474e-94ba-317d3b1ba18a', true, true),
    ('fdf75e72-c0a8-4b47-8103-1f8e2991b3f2', $1, 'Ид свяьза', 'view_relation_permission', '076c519a-5503-4bff-99f1-c741ed7d47b8', true, true),
    ('46c51357-368d-477e-94bd-c4ae1c79bc20', $1, 'Разрешение на просмотр', 'view_relation_permission', 'c5962e1c-2687-46a5-b2dd-d46d41a038c1', true, true),
    ('3bc850eb-ac68-4a5f-9403-33e3b638ce48', $1, 'ID', 'view_relation_permission', 'a73fd453-3c21-4ab8-9e21-59d85acd106c', true, true),
    ('0f721af1-5a6f-4162-8678-b7985048f019', $1, 'Full Name', 'person', 'f54d8076-4972-4067-9a91-c178c02c4273', true, true),
    ('3f48429e-b0f2-4455-8811-4f4cb7bc4d2e', $1, 'Email', 'person', 'd868638d-35d6-4992-8216-7b2f479f722e', true, true),
    ('6c8bce87-ee38-43b6-aed7-2ceec6296eb2', $1, 'Phone Number', 'person', 'eb3deeb7-6d34-4e24-b65a-f03e09efd0cf', true, true),
    ('cb5e135b-5956-4a47-8114-82d9c597d803', $1, 'Gender', 'person', 'b92b9b8c-c138-4ce6-9260-b4452a7f5ae2', true, true),
    ('58071152-a906-432a-a87e-1696439ceb85', $1, 'Image', 'person', 'c5b09b80-528d-4987-9105-a2be539255ee', true, true),
    ('aceb8201-6d90-49e0-9951-a024b9b0ce41', $1, 'Date Of Birth', 'person', 'e5a2a21e-a9e2-4e6d-87e8-57b8dd837d48', true, true),
    ('15debca3-1097-450b-9e9f-7a98281b4ae7', $1, 'User ID Auth', 'person', '4342bf9d-24ad-4f74-bb7e-156d3b4e1dfd', true, true),
    ('9c2169c1-438e-4fb7-8cf7-ead729480e96', $1, 'FROM person TO client_type', 'person', '2b1549b1-5490-41f3-8442-d3e116d847fb', true, true),
    ('0c28a001-6813-4cd2-9cf8-e395412e79f3', $1, 'FROM person TO role', 'person', '84d98d58-1a8f-4916-8bd0-3b0d0b6bfb0a', true, true),
    ('f63ef9fa-e3dc-49f6-b4f6-4b912f08fd5f', $1, 'Login', 'person', '4f7ade49-da8a-4534-b3a4-35f2875609b1', true, true),
    ('c37b5a14-78b5-4a74-b6c1-8f2f1dc910be', $1, 'Password', 'person', '88ef053a-ae80-44a0-aad1-055f4405a3ee', true, true);`

	_, err := conn.Exec(context.Background(), query, roleId)
	if err != nil {
		return err
	}

	return nil
}
