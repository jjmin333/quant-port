-- 로컬 MySQL 초기 설정 (root 계정으로 실행)
-- MySQL Workbench, HeidiSQL, 또는 mysql CLI에서 실행하세요.

CREATE DATABASE IF NOT EXISTS quantport
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;

CREATE USER IF NOT EXISTS 'quantport'@'localhost' IDENTIFIED BY 'quantport';
GRANT ALL PRIVILEGES ON quantport.* TO 'quantport'@'localhost';
FLUSH PRIVILEGES;

-- 스키마는 BE 시작 시 migrations/ 가 자동 적용됩니다.
-- 수동 적용: be/migrations/*.sql 순서대로 실행
