DELETE FROM fare_item_price WHERE id IN (
    '61bb7c07-9c84-4de0-8c99-b9ed8bc4c1a4','9c04ff8c-f635-492c-8620-44b10bad942b','af4c7be0-489c-4a06-b09f-1db4458adbc6',
    '9947c85d-cd52-45c7-aac8-ae6285975c6b','36dec5c1-4dc1-4c53-8905-edd19fa6423a','d2b062e1-0e9f-4a75-878c-52feef69c2d1',
    'e161ce1e-1712-481c-a7b0-b01660037742','6c6227dc-3a24-45bb-aa72-96cc04fc3af5','f6d1fc0e-2841-4813-bdcc-171525b16061',
    '7093ece2-4977-4447-ac69-f46460caad46','0d18ebc4-7628-472c-aba5-e71eb4197f75','0e6826d0-f2f3-43cb-a6fc-2d87d13b9679',
    'a349bc04-965d-4ed0-a2c4-20c97578a8ea','3262c8b8-adfe-4bbd-9c6a-2b9b2de1951a','89d0fa1d-ae55-4398-b534-d31ff5c65ff0'
);

DELETE FROM fare_item WHERE id IN (
    'f3cb57f8-0571-4775-b819-c46f16357e59',
    '0d6fda97-c8fe-4c8e-b949-2853268f95d2',
    '95fc7ebf-2a79-40af-b20c-0b9d53fc51f5',
    '3f6ee755-6ad5-4c50-8970-17eeab3f086c'
);

UPDATE fare_item SET type = 'database' WHERE id = '1873b655-af22-40c8-a94e-ff577f818bc5';
