package mendoza

import (
	"fmt"
	"sort"
)

type outputEntry struct {
	key            string
	source         interface{}
	writableArray  []interface{}
	writableObject map[string]interface{}
}

type inputEntry struct {
	value  interface{}
	fields []fieldEntry
}

type fieldEntry struct {
	key   string
	value interface{}
}

type patcher struct {
	root        interface{}
	inputStack  []inputEntry
	outputStack []outputEntry
}

// TODO: Different name?

func Decode(root interface{}, patch Patch) interface{} {
	p := patcher{
		root: root,
	}

	for _, op := range patch {
		p.process(op)
	}

	return p.result()
}

func (patcher *patcher) enter(enterType EnterType, value interface{}, key string) {
	patcher.inputStack = append(patcher.inputStack, inputEntry{
		value: value,
	})

	switch enterType {
	case EnterNop:
		// do nothing
	case EnterCopy:
		patcher.outputStack = append(patcher.outputStack, outputEntry{
			key:    key,
			source: value,
		})
	case EnterBlank:
		patcher.outputStack = append(patcher.outputStack, outputEntry{
			key: key,
		})
	}
}

func (patcher *patcher) returnIntoField(key string) {
	patcher.inputStack = patcher.inputStack[:len(patcher.inputStack)-1]

	// Read the current value, then pop the stack
	entry := patcher.outputStack[len(patcher.outputStack)-1]
	patcher.outputStack = patcher.outputStack[:len(patcher.outputStack)-1]

	obj := patcher.outputObject()

	if key == "" {
		key = entry.key
	}

	obj[key] = entry.result()
}

func (patcher *patcher) returnIntoArray() {
	patcher.inputStack = patcher.inputStack[:len(patcher.inputStack)-1]

	// Read the current value, then pop the stack
	entry := patcher.outputStack[len(patcher.outputStack)-1]
	patcher.outputStack = patcher.outputStack[:len(patcher.outputStack)-1]

	arr := patcher.outputArray()
	*arr = append(*arr, entry.result())
}

func (patcher *patcher) inputEntry() *inputEntry {
	return &patcher.inputStack[len(patcher.inputStack)-1]
}

func (entry *outputEntry) result() interface{} {
	if entry.writableObject != nil {
		return entry.writableObject
	}

	if entry.writableArray != nil {
		return entry.writableArray
	}

	return entry.source
}

func (entry *inputEntry) getField(idx int) fieldEntry {
	if entry.fields == nil {
		fields := []fieldEntry{}
		obj := entry.value.(map[string]interface{})
		keys := []string{}
		for key := range obj {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			val := obj[key]
			fields = append(fields, fieldEntry{
				key:   key,
				value: val,
			})
		}
		entry.fields = fields
	}

	return entry.fields[idx]
}

func (patcher *patcher) inputObject() map[string]interface{} {
	return patcher.inputEntry().value.(map[string]interface{})
}

func (patcher *patcher) inputArray() []interface{} {
	return patcher.inputEntry().value.([]interface{})
}

func (patcher *patcher) result() interface{} {
	entry := patcher.outputStack[len(patcher.outputStack)-1]
	return entry.result()
}

func (patcher *patcher) outputObject() map[string]interface{} {
	entry := &patcher.outputStack[len(patcher.outputStack)-1]

	if entry.writableObject == nil {
		src := entry.source.(map[string]interface{})
		obj := make(map[string]interface{}, len(src))

		for k, v := range src {
			obj[k] = v
		}

		entry.writableObject = obj
	}

	return entry.writableObject
}

func (patcher *patcher) outputArray() *[]interface{} {
	entry := &patcher.outputStack[len(patcher.outputStack)-1]

	if entry.source != nil {
		src := entry.source.([]interface{})
		entry.writableArray = make([]interface{}, len(src))
		copy(entry.writableArray, src)
		entry.source = nil
	}

	return &entry.writableArray
}

func (patcher *patcher) process(op Op) {
	switch op := op.(type) {
	case OpEnterValue:
		patcher.outputStack = append(patcher.outputStack, outputEntry{
			source: op.Value,
		})
	case OpEnterRoot:
		patcher.enter(op.Enter, patcher.root, "")
	case OpEnterField:
		field := patcher.inputEntry().getField(op.Index)
		patcher.enter(op.Enter, field.value, field.key)
	case OpEnterElement:
		arr := patcher.inputArray()
		value := arr[op.Index]
		patcher.enter(op.Enter, value, "")
	case OpReturnIntoObject:
		patcher.returnIntoField(op.Key)
	case OpReturnIntoArray:
		patcher.returnIntoArray()
	case OpObjectSetFieldValue:
		obj := patcher.outputObject()
		obj[op.Key] = op.Value
	case OpObjectCopyField:
		field := patcher.inputEntry().getField(op.Index)
		obj := patcher.outputObject()
		obj[field.key] = field.value
	case OpObjectDeleteField:
		field := patcher.inputEntry().getField(op.Index)
		obj := patcher.outputObject()
		delete(obj, field.key)
	case OpArrayAppendValue:
		arr := patcher.outputArray()
		*arr = append(*arr, op.Value)
	case OpArrayAppendSlice:
		src := patcher.inputArray()
		arr := patcher.outputArray()
		*arr = append(*arr, src[op.Left:op.Right]...)
	default:
		panic(fmt.Errorf("unknown op: %#v", op))
	}
}
