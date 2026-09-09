-- Delete ugen fares (cascades to their fare_item_price records)
DELETE FROM fare WHERE id IN (
    '07d8a364-ebb2-4291-a452-f44b335cb031',
    '26b264b7-abad-4813-87c0-0d95e6cf226a',
    '16decb16-1939-4502-a219-1cfd44480442'
);

-- Delete new fare_items (cascades to fare_item_price)
DELETE FROM fare_item WHERE id IN (
    'b1d849e1-7c06-4281-9f36-7f01778c6f23',
    'e7296d25-163a-4116-a392-9b14a4c7add2',
    '6482ece4-31cf-4d7d-8338-61673a1e2efb',
    'ebc2c55d-b61f-4db3-94f3-57edc2ab834b'
);
