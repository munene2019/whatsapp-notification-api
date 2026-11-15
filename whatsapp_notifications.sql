-- phpMyAdmin SQL Dump
-- version 5.2.1
-- https://www.phpmyadmin.net/
--
-- Host: 127.0.0.1:3307
-- Generation Time: Nov 15, 2026 at 05:29 PM
-- Server version: 10.4.32-MariaDB
-- PHP Version: 8.0.30

SET SQL_MODE = "NO_AUTO_VALUE_ON_ZERO";
START TRANSACTION;
SET time_zone = "+00:00";


/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!40101 SET NAMES utf8mb4 */;

--
-- Database: `whatsapp_notifications`
--

-- --------------------------------------------------------

--
-- Table structure for table `message_history`
--

CREATE TABLE `message_history` (
  `id` bigint(20) NOT NULL,
  `recipient` varchar(20) NOT NULL,
  `template_name` varchar(255) DEFAULT NULL,
  `language_code` varchar(20) DEFAULT NULL,
  `message_type` varchar(30) NOT NULL,
  `delivery_mode` varchar(20) NOT NULL,
  `payload` longtext DEFAULT NULL,
  `scheduled_message_id` bigint(20) DEFAULT NULL,
  `scheduled_at` datetime(6) DEFAULT NULL,
  `submitted_at` datetime(6) DEFAULT NULL,
  `sent_at` datetime(6) DEFAULT NULL,
  `delivered_at` datetime(6) DEFAULT NULL,
  `read_at` datetime(6) DEFAULT NULL,
  `provider_message_id` varchar(255) DEFAULT NULL,
  `status` varchar(30) NOT NULL DEFAULT 'pending',
  `error_message` text DEFAULT NULL,
  `created_at` datetime(6) NOT NULL DEFAULT current_timestamp(6),
  `updated_at` datetime(6) NOT NULL DEFAULT current_timestamp(6) ON UPDATE current_timestamp(6)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

--
-- Dumping data for table `message_history`
--

INSERT INTO `message_history` (`id`, `recipient`, `template_name`, `language_code`, `message_type`, `delivery_mode`, `payload`, `scheduled_message_id`, `scheduled_at`, `submitted_at`, `sent_at`, `delivered_at`, `read_at`, `provider_message_id`, `status`, `error_message`, `created_at`, `updated_at`) VALUES
(1, '254712622099', 'hello_world', 'en_US', 'template', 'scheduled', '{\"messaging_product\":\"whatsapp\",\"to\":\"254712622099\",\"type\":\"template\",\"template\":{\"name\":\"hello_world\",\"language\":{\"code\":\"en_US\"}}}', 1, '2026-11-15 15:58:28.000000', '2026-11-15 15:58:29.788716', '2026-11-15 15:58:31.000000', '2026-11-15 15:58:31.000000', NULL, 'wamid.HBgMMjU0NzEyNjIyMDk5FQIAERgSQTFGNTZCQzAzOEM4RTBDRjQ5AA==', 'delivered', NULL, '2026-11-15 18:56:36.046062', '2026-11-15 18:58:31.693759'),
(2, '254712622099', 'hello_world', 'en_US', 'template', 'immediate', '{\"messaging_product\":\"whatsapp\",\"to\":\"254712622099\",\"type\":\"template\",\"template\":{\"name\":\"hello_world\",\"language\":{\"code\":\"en_US\"}}}', NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'failed', 'WhatsApp API returned HTTP 401: {\"error\":{\"message\":\"Authentication Error\",\"code\":190,\"type\":\"OAuthException\",\"fbtrace_id\":\"ALy_chqYjd7zsteNFDRJU0O\"}}', '2026-11-15 19:00:33.915129', '2026-11-15 19:00:34.635994'),
(3, '254712622099', 'hello_world', 'en_US', 'template', 'immediate', '{\"messaging_product\":\"whatsapp\",\"to\":\"254712622099\",\"type\":\"template\",\"template\":{\"name\":\"hello_world\",\"language\":{\"code\":\"en_US\"}}}', NULL, NULL, NULL, NULL, NULL, NULL, NULL, 'failed', 'WhatsApp API returned HTTP 401: {\"error\":{\"message\":\"Authentication Error\",\"code\":190,\"type\":\"OAuthException\",\"fbtrace_id\":\"AICYq7sgQJKESwCHK1Crlcy\"}}', '2026-11-15 19:00:53.012810', '2026-11-15 19:00:53.260793'),
(4, '254712622099', 'hello_world', 'en_US', 'template', 'immediate', '{\"messaging_product\":\"whatsapp\",\"to\":\"254712622099\",\"type\":\"template\",\"template\":{\"name\":\"hello_world\",\"language\":{\"code\":\"en_US\"}}}', NULL, NULL, '2026-11-15 16:02:05.903135', '2026-11-15 16:02:07.000000', '2026-11-15 16:02:07.000000', NULL, 'wamid.HBgMMjU0NzEyNjIyMDk5FQIAERgSN0M1MjlCMzdDRjZBNUMzMzk0AA==', 'delivered', NULL, '2026-11-15 19:02:04.224221', '2026-11-15 19:02:07.567680'),
(5, '254712622099', 'hello_world', 'en_US', 'template', 'immediate', '{\"messaging_product\":\"whatsapp\",\"to\":\"254712622099\",\"type\":\"template\",\"template\":{\"name\":\"hello_world\",\"language\":{\"code\":\"en_US\"}}}', NULL, NULL, '2026-11-15 16:02:28.459802', '2026-11-15 16:02:29.000000', '2026-11-15 16:02:30.000000', NULL, 'wamid.HBgMMjU0NzEyNjIyMDk5FQIAERgSN0Q5RDVEQzMyRUNCMzE5MzEyAA==', 'delivered', NULL, '2026-11-15 19:02:27.158967', '2026-11-15 19:02:30.361703');

-- --------------------------------------------------------

--
-- Table structure for table `scheduled_messages`
--

CREATE TABLE `scheduled_messages` (
  `id` bigint(20) NOT NULL,
  `recipient` varchar(20) NOT NULL,
  `template_name` varchar(255) NOT NULL,
  `language_code` varchar(20) NOT NULL,
  `payload` longtext CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL CHECK (json_valid(`payload`)),
  `scheduled_at` datetime(6) NOT NULL,
  `status` varchar(20) NOT NULL DEFAULT 'pending',
  `attempts` int(11) NOT NULL DEFAULT 0,
  `error_message` text DEFAULT NULL,
  `provider_message_id` varchar(255) DEFAULT NULL,
  `created_at` datetime(6) NOT NULL DEFAULT current_timestamp(6),
  `updated_at` datetime(6) NOT NULL DEFAULT current_timestamp(6) ON UPDATE current_timestamp(6)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

--
-- Dumping data for table `scheduled_messages`
--

INSERT INTO `scheduled_messages` (`id`, `recipient`, `template_name`, `language_code`, `payload`, `scheduled_at`, `status`, `attempts`, `error_message`, `provider_message_id`, `created_at`, `updated_at`) VALUES
(1, '254712622099', 'hello_world', 'en_US', '{\"messaging_product\":\"whatsapp\",\"to\":\"254712622099\",\"type\":\"template\",\"template\":{\"name\":\"hello_world\",\"language\":{\"code\":\"en_US\"}}}', '2026-11-15 15:58:28.000000', 'sent', 1, NULL, 'wamid.HBgMMjU0NzEyNjIyMDk5FQIAERgSQTFGNTZCQzAzOEM4RTBDRjQ5AA==', '2026-11-15 18:56:36.045133', '2026-11-15 18:58:29.788072');

--
-- Indexes for dumped tables
--

--
-- Indexes for table `message_history`
--
ALTER TABLE `message_history`
  ADD PRIMARY KEY (`id`),
  ADD UNIQUE KEY `provider_message_id` (`provider_message_id`),
  ADD KEY `idx_recipient` (`recipient`),
  ADD KEY `idx_status` (`status`),
  ADD KEY `idx_created_at` (`created_at`);

--
-- Indexes for table `scheduled_messages`
--
ALTER TABLE `scheduled_messages`
  ADD PRIMARY KEY (`id`),
  ADD KEY `idx_scheduled_status_time` (`status`,`scheduled_at`);

--
-- AUTO_INCREMENT for dumped tables
--

--
-- AUTO_INCREMENT for table `message_history`
--
ALTER TABLE `message_history`
  MODIFY `id` bigint(20) NOT NULL AUTO_INCREMENT, AUTO_INCREMENT=6;

--
-- AUTO_INCREMENT for table `scheduled_messages`
--
ALTER TABLE `scheduled_messages`
  MODIFY `id` bigint(20) NOT NULL AUTO_INCREMENT, AUTO_INCREMENT=2;
COMMIT;

/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
