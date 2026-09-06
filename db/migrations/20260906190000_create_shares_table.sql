-- migrate:up
-- A share is a read grant handed out as a link: one row per shared thing, and
-- the row is the whole of the grant. Revoking is deleting it, and there is no
-- second state to get out of step with the first.
--
-- token IS THE CREDENTIAL AND id IS NOT. Every other identifier in this schema
-- is a ULID that travels in URLs the owner is already authorised for; this one
-- travels in a link handed to strangers, and whoever holds it is let in. So it
-- is its own column: 128 random bits, base64url, and rotatable without
-- destroying the row that the entry, the expiry and the password hang off.
--
-- ascii_bin ON BOTH TEXT COLUMNS, and on token it is load-bearing rather than
-- tidy. base64url distinguishes `a` from `A`, so under the table's default
-- case-insensitive collation two different tokens could collide on the unique
-- index and a lookup could match a link that was never issued.
--
-- resource_type is an enum of one because the second member is the point of
-- having it: a character sheet or an inventory list shared later is a row in
-- this table rather than a table of its own. It is in the unique key so
-- "one live link per thing" is the database's rule and not the handler's.
--
-- character_id IS DENORMALISED, AND DELIBERATELY. It is derivable -- the
-- journal row names the character -- but deleting a character has to be able
-- to find and delete these rows, and reaching them through journals means a
-- subquery against a table that same request is emptying. It also keeps this
-- table visible to the schema-driven purge test, which finds what a character
-- delete owes by looking for exactly this column name.
--
-- password_hash is bcrypt, which is always 60 ASCII bytes. NULL is the common
-- case: a link with no password at all.
--
-- expires_at IS ABSOLUTE, not the day count the form collects. The form asks
-- "how many days", this stores the instant that lands on, and every read
-- checks it -- so a share stops working the moment it expires whether or not
-- anything has swept it yet. NULL never expires.
--
-- No FOREIGN KEY, like everything else here. Nothing cascades in this schema
-- and the deletes are written out where they happen.
CREATE TABLE shares (
    id VARBINARY(16) NOT NULL,
    owner_id VARBINARY(16) NOT NULL,
    character_id VARBINARY(16) NOT NULL,
    resource_type ENUM('journal') NOT NULL DEFAULT 'journal',
    resource_id VARBINARY(16) NOT NULL,
    token CHAR(22) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    password_hash VARCHAR(60) CHARACTER SET ascii COLLATE ascii_bin NULL,
    expires_at DATETIME NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY ux_shares_token (token),
    UNIQUE KEY ux_shares_resource (resource_type, resource_id),
    KEY idx_shares_character (character_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- migrate:down
DROP TABLE shares;
