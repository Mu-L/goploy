// Copyright 2022 The Goploy Authors. All rights reserved.
// Use of this source code is governed by a GPLv3-style
// license that can be found in the LICENSE file.

package model

import (
	"encoding/json"
	"time"

	sq "github.com/Masterminds/squirrel"
)

const monitorTable = "`monitor`"

type Monitor struct {
	ID              int64         `json:"id"`
	NamespaceID     int64         `json:"namespaceId"`
	Name            string        `json:"name"`
	Type            int           `json:"type"`
	Target          MonitorTarget `json:"target"`
	Second          int           `json:"second"`
	Times           uint16        `json:"times"`
	SilentCycle     int           `json:"silentCycle"`
	NotifyType      uint8         `json:"notifyType"`
	NotifyTarget    string        `json:"notifyTarget"`
	SuccessServerID int64         `json:"successServerId"`
	SuccessScript   string        `json:"successScript"`
	FailServerID    int64         `json:"failServerId"`
	FailScript      string        `json:"failScript"`
	Description     string        `json:"description"`
	ErrorContent    string        `json:"errorContent"`
	State           uint8         `json:"state"`
	InsertTime      string        `json:"insertTime"`
	UpdateTime      string        `json:"updateTime"`
}

type MonitorTarget struct {
	Items   []string      `json:"items"`
	Timeout time.Duration `json:"timeout"`
	Process string        `json:"process"`
	Script  string        `json:"script"`
}

type Monitors []Monitor

func (m Monitor) GetList() (Monitors, error) {
	rows, err := sq.
		Select("id, name, type, target, second, times, silent_cycle, notify_type, notify_target, description, error_content, state, success_script, fail_script, success_server_id, fail_server_id, insert_time, update_time").
		From(monitorTable).
		Where(sq.Eq{
			"namespace_id": m.NamespaceID,
		}).
		OrderBy("id DESC").
		RunWith(DB).
		Query()
	if err != nil {
		return nil, err
	}
	monitors := Monitors{}
	for rows.Next() {
		var monitor Monitor
		var target []byte
		if err := rows.Scan(
			&monitor.ID,
			&monitor.Name,
			&monitor.Type,
			&target,
			&monitor.Second,
			&monitor.Times,
			&monitor.SilentCycle,
			&monitor.NotifyType,
			&monitor.NotifyTarget,
			&monitor.Description,
			&monitor.ErrorContent,
			&monitor.State,
			&monitor.SuccessScript,
			&monitor.FailScript,
			&monitor.SuccessServerID,
			&monitor.FailServerID,
			&monitor.InsertTime,
			&monitor.UpdateTime); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(target, &monitor.Target); err != nil {
			return nil, err
		}
		monitors = append(monitors, monitor)
	}

	return monitors, nil
}

func (m Monitor) GetData() (Monitor, error) {
	var monitor Monitor
	var target []byte
	err := sq.
		Select("id, name, type, target, second, times, silent_cycle, notify_type, notify_target, state", "success_script", "fail_script", "success_server_id", "fail_server_id").
		From(monitorTable).
		Where(sq.Eq{"id": m.ID}).
		OrderBy("id DESC").
		RunWith(DB).
		QueryRow().
		Scan(&monitor.ID, &monitor.Name, &monitor.Type, &target, &monitor.Second, &monitor.Times, &monitor.SilentCycle, &monitor.NotifyType, &monitor.NotifyTarget, &monitor.State, &monitor.SuccessScript, &monitor.FailScript, &monitor.SuccessServerID, &monitor.FailServerID)
	if err != nil {
		return monitor, err
	}

	if err := json.Unmarshal(target, &monitor.Target); err != nil {
		return monitor, err
	}
	return monitor, nil
}

func (m Monitor) GetAllByState() (Monitors, error) {
	rows, err := sq.
		Select("id, name, type, target, second, times, silent_cycle, notify_type, notify_target, success_script, fail_script, success_server_id, fail_server_id, description, update_time").
		From(monitorTable).
		Where(sq.Eq{
			"state": m.State,
		}).
		RunWith(DB).
		Query()
	if err != nil {
		return nil, err
	}
	monitors := Monitors{}
	for rows.Next() {
		var monitor Monitor
		var target []byte
		if err := rows.Scan(
			&monitor.ID,
			&monitor.Name,
			&monitor.Type,
			&target,
			&monitor.Second,
			&monitor.Times,
			&monitor.SilentCycle,
			&monitor.NotifyType,
			&monitor.NotifyTarget,
			&monitor.SuccessScript,
			&monitor.FailScript,
			&monitor.SuccessServerID,
			&monitor.FailServerID,
			&monitor.Description,
			&monitor.UpdateTime,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(target, &monitor.Target); err != nil {
			return nil, err
		}
		monitors = append(monitors, monitor)
	}

	return monitors, nil
}

func (m Monitor) AddRow() (int64, error) {
	target, err := json.Marshal(m.Target)
	if err != nil {
		return 0, err
	}
	result, err := sq.
		Insert(monitorTable).
		Columns("namespace_id", "name", "type", "target", "second", "times", "silent_cycle", "notify_type", "notify_target", "description", "error_content", "success_script", "fail_script", "success_server_id", "fail_server_id").
		Values(m.NamespaceID, m.Name, m.Type, target, m.Second, m.Times, m.SilentCycle, m.NotifyType, m.NotifyTarget, m.Description, "", m.SuccessScript, m.FailScript, m.SuccessServerID, m.FailServerID).
		RunWith(DB).
		Exec()
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	return id, err
}

func (m Monitor) EditRow() error {
	target, err := json.Marshal(m.Target)
	if err != nil {
		return err
	}
	_, err = sq.
		Update(monitorTable).
		SetMap(sq.Eq{
			"name":              m.Name,
			"type":              m.Type,
			"target":            target,
			"second":            m.Second,
			"times":             m.Times,
			"silent_cycle":      m.SilentCycle,
			"notify_type":       m.NotifyType,
			"notify_target":     m.NotifyTarget,
			"description":       m.Description,
			"success_script":    m.SuccessScript,
			"fail_script":       m.FailScript,
			"success_server_id": m.SuccessServerID,
			"fail_server_id":    m.FailServerID,
		}).
		Where(sq.Eq{"id": m.ID}).
		RunWith(DB).
		Exec()
	return err
}

func (m Monitor) ToggleState() error {
	_, err := sq.
		Update(monitorTable).
		SetMap(sq.Eq{
			"state": m.State,
		}).
		Where(sq.Eq{"id": m.ID}).
		RunWith(DB).
		Exec()
	return err
}

func (m Monitor) DeleteRow() error {
	_, err := sq.
		Delete(monitorTable).
		Where(sq.Eq{"id": m.ID}).
		RunWith(DB).
		Exec()
	return err
}

func (m Monitor) UpdateLatestErrorContent(errorContent string) error {
	_, err := sq.
		Update(monitorTable).
		SetMap(sq.Eq{
			"error_content": errorContent,
		}).
		Where(sq.Eq{"id": m.ID}).
		RunWith(DB).
		Exec()
	return err
}
