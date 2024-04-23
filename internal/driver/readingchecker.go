// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2019 IOTech Ltd
// Copyright (C) 2024 YIQISOFT
// Copyright (C) 2024 liushenglong_8597@outlook.com
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"fmt"
	"math"
	"time"

	sdkModel "github.com/edgexfoundry/device-sdk-go/v3/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/common"
	"github.com/spf13/cast"
)

func newResult(req *CommandInfo, reading interface{}) (*sdkModel.CommandValue, error) {
	var result = &sdkModel.CommandValue{}
	var err error
	castError := "fail to parse %v reading, %v"

	if !checkValueInRange(req.valueType, reading) {
		err = fmt.Errorf("parse reading fail. Reading %v is out of the value type(%v)'s range", reading, req.valueType)
		return result, err
	}

	var val interface{}

	switch req.valueType {
	case common.ValueTypeBool:
		val, err = cast.ToBoolE(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.resourceName, err)
		}
	case common.ValueTypeString:
		val, err = cast.ToStringE(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.resourceName, err)
		}
	case common.ValueTypeUint8:
		val, err = cast.ToUint8E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.resourceName, err)
		}
	case common.ValueTypeUint16:
		val, err = cast.ToUint16E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.resourceName, err)
		}
	case common.ValueTypeUint32:
		val, err = cast.ToUint32E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.resourceName, err)
		}
	case common.ValueTypeUint64:
		val, err = cast.ToUint64E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.resourceName, err)
		}
	case common.ValueTypeInt8:
		val, err = cast.ToInt8E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.resourceName, err)
		}
	case common.ValueTypeInt16:
		val, err = cast.ToInt16E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.resourceName, err)
		}
	case common.ValueTypeInt32:
		val, err = cast.ToInt32E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.resourceName, err)
		}
	case common.ValueTypeInt64:
		val, err = cast.ToInt64E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.resourceName, err)
		}
	case common.ValueTypeFloat32:
		val, err = cast.ToFloat32E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.resourceName, err)
		}
	case common.ValueTypeFloat64:
		val, err = cast.ToFloat64E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.resourceName, err)
		}
	default:
		err = fmt.Errorf("return result fail, none supported value type: %v", req.valueType)
		return nil, err
	}

	result, err = sdkModel.NewCommandValue(req.resourceName, req.valueType, val)
	if err != nil {
		return nil, err
	}
	result.Origin = time.Now().UnixNano() / int64(time.Millisecond)

	return result, err
}

// checkValueInRange checks value range is valid
func checkValueInRange(valueType string, reading interface{}) bool {
	isValid := false

	if valueType == common.ValueTypeString || valueType == common.ValueTypeBool {
		return true
	}

	if valueType == common.ValueTypeInt8 || valueType == common.ValueTypeInt16 ||
		valueType == common.ValueTypeInt32 || valueType == common.ValueTypeInt64 {
		val := cast.ToInt64(reading)
		isValid = checkIntValueRange(valueType, val)
	}

	if valueType == common.ValueTypeUint8 || valueType == common.ValueTypeUint16 ||
		valueType == common.ValueTypeUint32 || valueType == common.ValueTypeUint64 {
		val := cast.ToUint64(reading)
		isValid = checkUintValueRange(valueType, val)
	}

	if valueType == common.ValueTypeFloat32 || valueType == common.ValueTypeFloat64 {
		val := cast.ToFloat64(reading)
		isValid = checkFloatValueRange(valueType, val)
	}

	return isValid
}

func checkUintValueRange(valueType string, val uint64) bool {
	var isValid = false
	switch valueType {
	case common.ValueTypeUint8:
		if val <= math.MaxUint8 {
			isValid = true
		}
	case common.ValueTypeUint16:
		if val <= math.MaxUint16 {
			isValid = true
		}
	case common.ValueTypeUint32:
		if val <= math.MaxUint32 {
			isValid = true
		}
	case common.ValueTypeUint64:
		maxiMum := uint64(math.MaxUint64)
		if val <= maxiMum {
			isValid = true
		}
	}
	return isValid
}

func checkIntValueRange(valueType string, val int64) bool {
	var isValid = false
	switch valueType {
	case common.ValueTypeInt8:
		if val >= math.MinInt8 && val <= math.MaxInt8 {
			isValid = true
		}
	case common.ValueTypeInt16:
		if val >= math.MinInt16 && val <= math.MaxInt16 {
			isValid = true
		}
	case common.ValueTypeInt32:
		if val >= math.MinInt32 && val <= math.MaxInt32 {
			isValid = true
		}
	case common.ValueTypeInt64:
		isValid = true
	}
	return isValid
}

func checkFloatValueRange(valueType string, val float64) bool {
	var isValid = false
	switch valueType {
	case common.ValueTypeFloat32:
		if !math.IsNaN(val) && math.Abs(val) <= math.MaxFloat32 {
			isValid = true
		}
	case common.ValueTypeFloat64:
		if !math.IsNaN(val) && !math.IsInf(val, 0) {
			isValid = true
		}
	}
	return isValid
}
