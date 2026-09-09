-- ucode api_keys were set to 0 (unlimited) in migration 68; set real limits
-- Small   (7d958abe): 5 api keys
-- Medium  (8c602ff5): 15 api keys
-- Private (81c98c75): 0 = unlimited (internal plan)
UPDATE fare_item_price SET value = '5'
WHERE fare_id = '7d958abe-08df-4735-a4f3-d275d1482b47' AND item_type = 'api_keys';

UPDATE fare_item_price SET value = '15'
WHERE fare_id = '8c602ff5-fd97-429a-870c-a6ebd46832c5' AND item_type = 'api_keys';