package dynamic

import "reflect"

// InheritStructValues merges the field value from the source struct to the dest struct.
// Only fields with the same type and the same name will be updated.
// Anonymous embedded struct fields are recursed into so that sub-fields
// (e.g. Quantity inside an embedded OpenPositionOptions) can be inherited
// from matching fields on the source.
func InheritStructValues(dst, src interface{}) {
	if dst == nil {
		return
	}

	dstVal := reflect.ValueOf(dst).Elem()
	srcVal := reflect.ValueOf(src).Elem()
	inheritFields(dstVal, srcVal)
}

func inheritFields(dstVal, srcVal reflect.Value) {
	dstType := dstVal.Type()
	srcType := srcVal.Type()

	for i := 0; i < dstType.NumField(); i++ {
		fieldType := dstType.Field(i)

		if !fieldType.IsExported() {
			continue
		}

		if fieldType.Anonymous && fieldType.Type.Kind() == reflect.Struct {
			inheritFields(dstVal.Field(i), srcVal)
			continue
		}

		fieldName := fieldType.Name

		fieldSrcType, found := srcType.FieldByName(fieldName)
		if !found {
			continue
		}

		if fieldSrcType.Type == fieldType.Type {
			srcFieldValue := srcVal.FieldByName(fieldName)
			dstFieldValue := dstVal.FieldByName(fieldName)
			if (fieldType.Type.Kind() == reflect.Ptr && dstFieldValue.IsNil()) || dstFieldValue.IsZero() {
				dstFieldValue.Set(srcFieldValue)
			}
		}
	}
}
