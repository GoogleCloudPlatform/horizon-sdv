-- Copyright 2025 Google LLC
--
-- Licensed under the Apache License, Version 2.0 (the "License");
-- you may not use this file except in compliance with the License.
-- You may obtain a copy of the License at
--
--     https://www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing, software
-- distributed under the License is distributed on an "AS IS" BASIS,
-- WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
-- See the License for the specific language governing permissions and
-- limitations under the License.

DROP INDEX IF EXISTS RefsRefIndex;
DROP TABLE IF EXISTS Refs;
DROP SEQUENCE IF EXISTS RefIdSequence;
DROP INDEX IF EXISTS ProjectsIndex;
DROP TABLE IF EXISTS Projects;
DROP SEQUENCE IF EXISTS ProjectIdSequence;
DROP INDEX IF EXISTS MapsFromIndex;
DROP INDEX IF EXISTS MapsToIndex;
DROP TABLE IF EXISTS Maps;
DROP SEQUENCE IF EXISTS MapIdSequence;
DROP TABLE IF EXISTS Chunks;
DROP SEQUENCE IF EXISTS ChunkIdSequence;
DROP TABLE IF EXISTS ChunkTouches;
DROP INDEX IF EXISTS LinksFromIndex;
DROP INDEX IF EXISTS LinksToIndex;
DROP TABLE IF EXISTS Links;
DROP SEQUENCE IF EXISTS LinkIdSequence;
DROP TABLE IF EXISTS Objects;
CREATE TABLE Objects (
    ObjectId STRING(128),
    AddTime TIMESTAMP NOT NULL,
    Type STRING(64) NOT NULL,
    Size INT64 NOT NULL,
    ContentEncoding STRING(64) NOT NULL,
    Content STRING(MAX) NOT NULL,
) PRIMARY KEY (ObjectId);
CREATE SEQUENCE LinkIdSequence OPTIONS (sequence_kind="bit_reversed_positive");
CREATE TABLE Links (
    LinkId INT64 DEFAULT (GET_NEXT_SEQUENCE_VALUE(SEQUENCE LinkIdSequence)),
    Index INT64 NOT NULL,
    `From` STRING(128) NOT NULL,
    `To` STRING(128) NOT NULL,
    CONSTRAINT FK_Links_From FOREIGN KEY (`From`) REFERENCES Objects (ObjectId) ON DELETE CASCADE,
    CONSTRAINT FK_Links_To FOREIGN KEY (`To`) REFERENCES Objects (ObjectId),
) PRIMARY KEY (LinkId);
CREATE INDEX LinksFromIndex ON Links(`From`);
CREATE INDEX LinksToIndex ON Links(`To`);
CREATE SEQUENCE ChunkIdSequence OPTIONS (sequence_kind="bit_reversed_positive");
CREATE TABLE Chunks (
    ChunkId INT64 DEFAULT (GET_NEXT_SEQUENCE_VALUE(SEQUENCE ChunkIdSequence)),
    AddedOn TIMESTAMP NOT NULL,
    Content BYTES(MAX) NOT NULL,
) PRIMARY KEY (ChunkId);
CREATE TABLE ChunkTouches (
    ChunkId INT64 NOT NULL,
    TouchedOn TIMESTAMP NOT NULL,
    Gen INT64 NOT NULL,
) PRIMARY KEY (ChunkId);
CREATE SEQUENCE MapIdSequence OPTIONS (sequence_kind="bit_reversed_positive");
CREATE TABLE Maps (
    MapId INT64 DEFAULT (GET_NEXT_SEQUENCE_VALUE(SEQUENCE MapIdSequence)),
    AddTime TIMESTAMP NOT NULL,
    `From` STRING(128) NOT NULL,
    `To` STRING(128) NOT NULL,
    CONSTRAINT FK_Maps_From FOREIGN KEY (`From`) REFERENCES Objects (ObjectId),
    CONSTRAINT FK_Maps_To FOREIGN KEY (`To`) REFERENCES Objects (ObjectId),
) PRIMARY KEY (MapId);
CREATE INDEX MapsFromIndex ON Maps(`From`);
CREATE INDEX MapsToIndex ON Maps(`To`);
CREATE SEQUENCE ProjectIdSequence OPTIONS (sequence_kind="bit_reversed_positive");
CREATE TABLE Projects (
    ProjectId INT64 DEFAULT (GET_NEXT_SEQUENCE_VALUE(SEQUENCE ProjectIdSequence)),
    Host STRING(MAX) NOT NULL,
    Project STRING(MAX) NOT NULL,
) PRIMARY KEY (ProjectId);
CREATE UNIQUE INDEX ProjectsIndex ON Projects(Host, Project);
CREATE SEQUENCE RefIdSequence OPTIONS (sequence_kind="bit_reversed_positive");
CREATE TABLE Refs (
    ProjectId INT64 NOT NULL,
    Ref STRING(MAX) NOT NULL,
    UpdateTime TIMESTAMP NOT NULL,
    Target STRING(128),
    CONSTRAINT FK_Refs_ProjectId FOREIGN KEY (ProjectId) REFERENCES Projects (ProjectId),
    CONSTRAINT FK_Refs_TargetId FOREIGN KEY (Target) REFERENCES Objects (ObjectId),
) PRIMARY KEY (ProjectId, Ref);
CREATE INDEX RefsRefIndex ON Refs(Ref);
