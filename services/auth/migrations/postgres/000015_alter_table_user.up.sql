alter table "user"
add constraint user_unq_login unique(login);

alter table "user"
add constraint user_unq_email unique(email);

alter table "user"
add constraint user_unq_phone unique(phone);