/*M!999999\- enable the sandbox mode */
-- MariaDB dump 10.19-12.3.2-MariaDB, for Linux (x86_64)
--
-- Host: 127.0.0.1    Database: tabletopper
-- ------------------------------------------------------
-- Server version	9.6.0

/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!40101 SET NAMES utf8mb4 */;
/*!40103 SET @OLD_TIME_ZONE=@@TIME_ZONE */;
/*!40103 SET TIME_ZONE='+00:00' */;
/*!40014 SET @OLD_UNIQUE_CHECKS=@@UNIQUE_CHECKS, UNIQUE_CHECKS=0 */;
/*!40014 SET @OLD_FOREIGN_KEY_CHECKS=@@FOREIGN_KEY_CHECKS, FOREIGN_KEY_CHECKS=0 */;
/*!40101 SET @OLD_SQL_MODE=@@SQL_MODE, SQL_MODE='NO_AUTO_VALUE_ON_ZERO' */;
/*M!100616 SET @OLD_NOTE_VERBOSITY=@@NOTE_VERBOSITY, NOTE_VERBOSITY=0 */;

--
-- Table structure for table `assets`
--

/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `assets` (
  `id` varbinary(16) NOT NULL,
  `owner_id` varbinary(16) NOT NULL,
  `journal_id` varbinary(16) DEFAULT NULL,
  `file_path` varchar(1024) NOT NULL,
  `preview_path` varchar(1024) DEFAULT NULL,
  `type` enum('map','avatar','token','music','journal','monster','character','profile','terrain') NOT NULL DEFAULT 'map',
  `file_name` varchar(512) NOT NULL,
  `size_bytes` bigint NOT NULL DEFAULT '0',
  `name` varchar(255) NOT NULL,
  `detached_at` datetime DEFAULT NULL,
  `width` int unsigned DEFAULT NULL,
  `height` int unsigned DEFAULT NULL,
  `tile_size` smallint unsigned DEFAULT NULL,
  `max_zoom` tinyint unsigned DEFAULT NULL,
  `tile_gen` varbinary(16) DEFAULT NULL,
  `tile_state` enum('pending','working','ready','failed') DEFAULT NULL,
  `tile_attempts` tinyint unsigned NOT NULL DEFAULT '0',
  `tile_lease` varbinary(16) DEFAULT NULL,
  `tile_leased_at` datetime DEFAULT NULL,
  `tiled_at` datetime DEFAULT NULL,
  `uploaded_at` datetime DEFAULT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_assets_owner` (`owner_id`),
  KEY `idx_assets_owner_type` (`owner_id`,`type`),
  KEY `idx_assets_journal` (`journal_id`),
  KEY `idx_assets_tile_state` (`tile_state`,`created_at`),
  KEY `idx_assets_pending_upload` (`type`,`uploaded_at`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `attacks`
--

/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `attacks` (
  `id` varbinary(16) NOT NULL,
  `owner_id` varbinary(16) NOT NULL,
  `character_id` varbinary(16) NOT NULL,
  `name` varchar(128) NOT NULL DEFAULT '',
  `attack_bonus` varchar(32) NOT NULL DEFAULT '',
  `damage` varchar(64) NOT NULL DEFAULT '',
  `damage_type` varchar(32) NOT NULL DEFAULT '',
  `mastery` varchar(32) NOT NULL DEFAULT '',
  `notes` text NOT NULL DEFAULT (_utf8mb4''),
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_attacks_character` (`character_id`,`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `characters`
--

/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `characters` (
  `id` varbinary(16) NOT NULL,
  `owner_id` varbinary(16) NOT NULL,
  `asset_id` varbinary(16) DEFAULT NULL,
  `name` varchar(128) NOT NULL,
  `level` tinyint unsigned NOT NULL DEFAULT '1',
  `xp` int unsigned NOT NULL DEFAULT '0',
  `race` varchar(64) DEFAULT NULL,
  `background` varchar(64) DEFAULT NULL,
  `alignment` varchar(32) DEFAULT NULL,
  `classes` varchar(64) DEFAULT NULL,
  `size` varchar(64) NOT NULL DEFAULT 'medium',
  `ac` smallint unsigned NOT NULL DEFAULT '10',
  `max_hp` smallint unsigned NOT NULL DEFAULT '1',
  `current_hp` smallint unsigned NOT NULL DEFAULT '1',
  `proficiency_bonus` smallint unsigned NOT NULL DEFAULT '2',
  `temp_hp` smallint unsigned NOT NULL DEFAULT '0',
  `speed` varchar(128) NOT NULL,
  `initiative_bonus` smallint NOT NULL DEFAULT '0',
  `str` tinyint unsigned NOT NULL,
  `dex` tinyint unsigned NOT NULL,
  `con` tinyint unsigned NOT NULL,
  `int` tinyint unsigned NOT NULL,
  `wis` tinyint unsigned NOT NULL,
  `cha` tinyint unsigned NOT NULL,
  `languages` varchar(255) NOT NULL,
  `proficiencies` varchar(255) NOT NULL,
  `skills` json NOT NULL,
  `saving_throws` json NOT NULL,
  `features` json NOT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `personality_traits` text NOT NULL DEFAULT (_utf8mb4''),
  `ideals` text NOT NULL DEFAULT (_utf8mb4''),
  `bonds` text NOT NULL DEFAULT (_utf8mb4''),
  `flaws` text NOT NULL DEFAULT (_utf8mb4''),
  `age` varchar(64) NOT NULL DEFAULT '',
  `height` varchar(64) NOT NULL DEFAULT '',
  `weight` varchar(64) NOT NULL DEFAULT '',
  `eyes` varchar(64) NOT NULL DEFAULT '',
  `skin` varchar(64) NOT NULL DEFAULT '',
  `hair` varchar(64) NOT NULL DEFAULT '',
  `hit_dice` varchar(64) NOT NULL DEFAULT '',
  `hit_dice_spent` tinyint unsigned NOT NULL DEFAULT '0',
  `death_save_successes` tinyint unsigned NOT NULL DEFAULT '0',
  `death_save_failures` tinyint unsigned NOT NULL DEFAULT '0',
  `heroic_inspiration` tinyint(1) NOT NULL DEFAULT '0',
  `exhaustion` tinyint unsigned NOT NULL DEFAULT '0',
  `skill_proficiencies` json NOT NULL DEFAULT (json_object()),
  `saving_throw_proficiencies` json NOT NULL DEFAULT (json_object()),
  `spellcasting_ability` enum('none','str','dex','con','int','wis','cha') NOT NULL DEFAULT 'none',
  `spell_bonus_misc` smallint NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  KEY `idx_characters_owner` (`owner_id`),
  KEY `idx_characters_owner_name` (`owner_id`,`name`),
  CONSTRAINT `chk_characters_death_saves` CHECK (((`death_save_successes` <= 3) and (`death_save_failures` <= 3))),
  CONSTRAINT `chk_characters_exhaustion` CHECK ((`exhaustion` <= 6)),
  CONSTRAINT `chk_characters_hit_dice_spent` CHECK ((`hit_dice_spent` <= 20))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `inventory`
--

/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `inventory` (
  `id` varbinary(16) NOT NULL,
  `owner_id` varbinary(16) NOT NULL,
  `character_id` varbinary(16) NOT NULL,
  `name` varchar(128) NOT NULL DEFAULT '',
  `quantity` int unsigned NOT NULL DEFAULT '1',
  `value` varchar(64) NOT NULL DEFAULT '',
  `weight` decimal(8,2) NOT NULL DEFAULT '0.00',
  `equipped` tinyint(1) NOT NULL DEFAULT '0',
  `description` text NOT NULL DEFAULT (_utf8mb4''),
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_inventory_character` (`character_id`,`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `journals`
--

/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `journals` (
  `id` varbinary(16) NOT NULL,
  `owner_id` varbinary(16) NOT NULL,
  `character_id` varbinary(16) NOT NULL,
  `title` varchar(255) NOT NULL DEFAULT '',
  `body` mediumtext NOT NULL DEFAULT (_utf8mb4''),
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_journals_character` (`character_id`,`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `monster_actions`
--

/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `monster_actions` (
  `id` varbinary(16) NOT NULL,
  `owner_id` varbinary(16) NOT NULL,
  `monster_id` varbinary(16) NOT NULL,
  `kind` enum('trait','action','bonus_action','reaction','legendary_action','lair_action','regional_effect') NOT NULL,
  `name` varchar(128) NOT NULL DEFAULT '',
  `description` text NOT NULL DEFAULT (_utf8mb4''),
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_monster_actions_monster` (`monster_id`,`kind`,`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `monsters`
--

/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `monsters` (
  `id` varbinary(16) NOT NULL,
  `owner_id` varbinary(16) NOT NULL,
  `asset_id` varbinary(16) DEFAULT NULL,
  `name` varchar(128) NOT NULL,
  `size` varchar(32) NOT NULL DEFAULT 'medium',
  `type` varchar(32) NOT NULL DEFAULT 'humanoid',
  `tags` varchar(128) NOT NULL DEFAULT '',
  `alignment` varchar(32) NOT NULL DEFAULT 'unaligned',
  `ac` tinyint unsigned NOT NULL DEFAULT '10',
  `hp` smallint unsigned NOT NULL DEFAULT '1',
  `hit_dice` varchar(64) NOT NULL DEFAULT '',
  `speed` varchar(128) NOT NULL DEFAULT '30 ft.',
  `initiative_bonus` smallint NOT NULL DEFAULT '0',
  `cr` varchar(4) NOT NULL DEFAULT '0',
  `legendary_action_uses` tinyint unsigned NOT NULL DEFAULT '0',
  `legendary_action_uses_in_lair` tinyint unsigned NOT NULL DEFAULT '0',
  `str` tinyint unsigned NOT NULL DEFAULT '10',
  `dex` tinyint unsigned NOT NULL DEFAULT '10',
  `con` tinyint unsigned NOT NULL DEFAULT '10',
  `int` tinyint unsigned NOT NULL DEFAULT '10',
  `wis` tinyint unsigned NOT NULL DEFAULT '10',
  `cha` tinyint unsigned NOT NULL DEFAULT '10',
  `skills` json NOT NULL DEFAULT (json_object()),
  `skill_proficiencies` json NOT NULL DEFAULT (json_object()),
  `saving_throws` json NOT NULL DEFAULT (json_object()),
  `saving_throw_proficiencies` json NOT NULL DEFAULT (json_object()),
  `vulnerabilities` varchar(512) NOT NULL DEFAULT '',
  `resistances` varchar(512) NOT NULL DEFAULT '',
  `immunities` varchar(512) NOT NULL DEFAULT '',
  `gear` varchar(512) NOT NULL DEFAULT '',
  `senses` varchar(255) NOT NULL DEFAULT '',
  `languages` varchar(255) NOT NULL DEFAULT '',
  `habitat` varchar(255) NOT NULL DEFAULT '',
  `treasure` varchar(64) NOT NULL DEFAULT '',
  `description` text NOT NULL DEFAULT (_utf8mb4''),
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_monsters_owner_name` (`owner_id`,`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `rooms`
--

/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `rooms` (
  `id` varbinary(16) NOT NULL,
  `owner_id` varbinary(16) NOT NULL,
  `name` varchar(128) NOT NULL,
  `code` char(4) CHARACTER SET ascii COLLATE ascii_general_ci DEFAULT NULL,
  `is_locked` tinyint(1) NOT NULL DEFAULT '0',
  `snapshot` json NOT NULL DEFAULT (json_object()),
  `snapshot_seq` bigint unsigned NOT NULL DEFAULT '0',
  `snapshot_at` datetime DEFAULT NULL,
  `snapshot_failed` json DEFAULT NULL,
  `snapshot_failed_at` datetime DEFAULT NULL,
  `scene_id` varbinary(16) DEFAULT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `closed_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_rooms_code` (`code`),
  KEY `idx_rooms_owner` (`owner_id`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `scenes`
--

/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `scenes` (
  `id` varbinary(16) NOT NULL,
  `owner_id` varbinary(16) NOT NULL,
  `name` varchar(128) NOT NULL,
  `body` json NOT NULL,
  `autosave` tinyint(1) NOT NULL DEFAULT '0',
  `preview_id` varbinary(16) DEFAULT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_scenes_owner` (`owner_id`,`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `schema_migrations`
--

/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `schema_migrations` (
  `version` varchar(128) NOT NULL,
  PRIMARY KEY (`version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `sessions`
--

/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `sessions` (
  `id` varbinary(16) NOT NULL,
  `hash` varbinary(32) NOT NULL,
  `user_id` varbinary(16) NOT NULL,
  `character_id` varbinary(16) DEFAULT NULL,
  `room_id` varbinary(16) DEFAULT NULL,
  `profile_image_url` varchar(512) NOT NULL DEFAULT '/images/default-avatar.webp',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `refreshed_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `expires_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_sessions_hash` (`hash`),
  KEY `idx_sessions_user_id` (`user_id`),
  KEY `idx_sessions_expires_at` (`expires_at`),
  KEY `idx_sessions_room_id` (`room_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `shares`
--

/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `shares` (
  `id` varbinary(16) NOT NULL,
  `owner_id` varbinary(16) NOT NULL,
  `character_id` varbinary(16) DEFAULT NULL,
  `resource_type` enum('journal','character','monster') NOT NULL DEFAULT 'journal',
  `resource_id` varbinary(16) NOT NULL,
  `token` char(22) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  `password_hash` varchar(60) CHARACTER SET ascii COLLATE ascii_bin DEFAULT NULL,
  `expires_at` datetime DEFAULT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_shares_token` (`token`),
  UNIQUE KEY `ux_shares_resource` (`resource_type`,`resource_id`),
  KEY `idx_shares_character` (`character_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `spell_slots`
--

/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `spell_slots` (
  `character_id` varbinary(16) NOT NULL,
  `owner_id` varbinary(16) NOT NULL,
  `level` tinyint unsigned NOT NULL,
  `slots` tinyint unsigned NOT NULL DEFAULT '0',
  `used` tinyint unsigned NOT NULL DEFAULT '0',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`character_id`,`level`),
  KEY `idx_spell_slots_owner` (`owner_id`),
  CONSTRAINT `chk_spell_slots_level` CHECK ((`level` <= 9))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `spells`
--

/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `spells` (
  `id` varbinary(16) NOT NULL,
  `owner_id` varbinary(16) NOT NULL,
  `character_id` varbinary(16) NOT NULL,
  `level` tinyint unsigned NOT NULL,
  `name` varchar(128) NOT NULL DEFAULT '',
  `school` varchar(32) NOT NULL DEFAULT 'Evocation',
  `components` varchar(128) NOT NULL DEFAULT '',
  `casting_time` varchar(64) NOT NULL DEFAULT '',
  `casting_range` varchar(64) NOT NULL DEFAULT '',
  `duration` varchar(64) NOT NULL DEFAULT '',
  `description` text NOT NULL DEFAULT (_utf8mb4''),
  `is_prepared` tinyint(1) NOT NULL DEFAULT '0',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_spells_character_level` (`character_id`,`level`,`id`),
  CONSTRAINT `chk_spells_level` CHECK ((`level` <= 9))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Table structure for table `users`
--

/*!40101 SET @saved_cs_client     = @@character_set_client */;
/*!40101 SET character_set_client = utf8mb4 */;
CREATE TABLE `users` (
  `id` varbinary(16) NOT NULL,
  `clerk_id` varchar(255) NOT NULL,
  `username` varchar(128) NOT NULL,
  `profile_image_url` varchar(512) NOT NULL DEFAULT '/images/default-avatar.webp',
  `avatar_asset_id` varbinary(16) DEFAULT NULL,
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `theme` enum('system','light','dark') NOT NULL DEFAULT 'system',
  `timezone` varchar(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'America/New_York',
  `date_format` enum('dmy_text','mdy_text','mdy_slash','dmy_slash','iso') NOT NULL DEFAULT 'dmy_text',
  `time_format` enum('12h','24h') NOT NULL DEFAULT '12h',
  `follow_turn` tinyint(1) NOT NULL DEFAULT '1',
  `show_blood` tinyint(1) NOT NULL DEFAULT '1',
  `ping_volume` tinyint unsigned NOT NULL DEFAULT '100',
  `turn_alert` tinyint(1) NOT NULL DEFAULT '1',
  `turn_volume` tinyint unsigned NOT NULL DEFAULT '100',
  `turn_notify` tinyint(1) NOT NULL DEFAULT '0',
  `onboarded_at` datetime DEFAULT NULL,
  `deleted_at` datetime DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_users_clerk_id` (`clerk_id`),
  KEY `idx_users_deleted` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
/*!40101 SET character_set_client = @saved_cs_client */;

--
-- Dumping routines for database 'tabletopper'
--
/*!40103 SET TIME_ZONE=@OLD_TIME_ZONE */;

/*!40101 SET SQL_MODE=@OLD_SQL_MODE */;
/*!40014 SET FOREIGN_KEY_CHECKS=@OLD_FOREIGN_KEY_CHECKS */;
/*!40014 SET UNIQUE_CHECKS=@OLD_UNIQUE_CHECKS */;
/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
/*M!100616 SET NOTE_VERBOSITY=@OLD_NOTE_VERBOSITY */;

-- Dump completed

--
-- Dbmate schema migrations
--

LOCK TABLES `schema_migrations` WRITE;
INSERT INTO `schema_migrations` (version) VALUES
  ('20260227164132'),
  ('20260227173035'),
  ('20260227175000'),
  ('20260227175838'),
  ('20260227181041'),
  ('20260227182710'),
  ('20260227185106'),
  ('20260905180000'),
  ('20260905180100'),
  ('20260905180200'),
  ('20260905180300'),
  ('20260905190000'),
  ('20260905200000'),
  ('20260905210000'),
  ('20260905210100'),
  ('20260905210200'),
  ('20260905220000'),
  ('20260905220100'),
  ('20260906120000'),
  ('20260906190000'),
  ('20260906200000'),
  ('20260906210000'),
  ('20260906220000'),
  ('20260906230000'),
  ('20260906240000'),
  ('20260906250000'),
  ('20260906260000'),
  ('20260906270000'),
  ('20260906280000'),
  ('20260906290000'),
  ('20260907120000'),
  ('20260907140000'),
  ('20260907150000'),
  ('20260907160000'),
  ('20260907170000'),
  ('20260909120000'),
  ('20260909130000'),
  ('20260909140000'),
  ('20260909150000'),
  ('20260910120000'),
  ('20260912120000'),
  ('20260913090000'),
  ('20260913120000'),
  ('20260913130000'),
  ('20261004120000');
UNLOCK TABLES;
