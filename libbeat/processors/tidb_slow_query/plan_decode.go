package convert

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/golang/snappy"
)

const (
	rootTaskType = "0"
	copTaskType  = "1"
)

const (
	idSeparator    = "_"
	lineBreaker    = '\n'
	lineBreakerStr = "\n"
	separator      = '\t'
	separatorStr   = "\t"
)

var (
	// PlanDiscardedEncoded indicates the discard plan because it is too long
	PlanDiscardedEncoded = "[discard]"
	planDiscardedDecoded = "(plan discarded because too long)"
)

var decoderPool = sync.Pool{
	New: func() interface{} {
		return &planDecoder{}
	},
}

// DecodePlan use to decode the string to plan tree.
func DecodePlan(planString string) (string, error) {
	if len(planString) == 0 {
		return "", nil
	}
	pd := decoderPool.Get().(*planDecoder)
	defer decoderPool.Put(pd)
	pd.buf.Reset()
	pd.addHeader = true
	return pd.decode(planString)
}

type planDecoder struct {
	buf              bytes.Buffer
	depths           []int
	indents          [][]rune
	planInfos        []*planInfo
	addHeader        bool
	cacheParentIdent map[int]int
}

type planInfo struct {
	depth  int
	fields []string
}

func (pd *planDecoder) decode(planString string) (string, error) {
	str, err := decompress(planString)
	if err != nil {
		if planString == PlanDiscardedEncoded {
			return planDiscardedDecoded, nil
		}
		return "", err
	}
	return pd.buildPlanTree(str)
}

func (pd *planDecoder) buildPlanTree(planString string) (string, error) {
	nodes := strings.Split(planString, lineBreakerStr)
	if len(pd.depths) < len(nodes) {
		pd.depths = make([]int, 0, len(nodes))
		pd.planInfos = make([]*planInfo, 0, len(nodes))
		pd.indents = make([][]rune, 0, len(nodes))
	}
	pd.depths = pd.depths[:0]
	pd.planInfos = pd.planInfos[:0]
	for _, node := range nodes {
		p, err := decodePlanInfo(node)
		if err != nil {
			return "", err
		}
		if p == nil {
			continue
		}
		pd.planInfos = append(pd.planInfos, p)
		pd.depths = append(pd.depths, p.depth)
	}

	if pd.addHeader {
		pd.addPlanHeader()
	}

	// Calculated indentation of plans.
	pd.initPlanTreeIndents()
	pd.cacheParentIdent = make(map[int]int)
	for i := 1; i < len(pd.depths); i++ {
		parentIndex := pd.findParentIndex(i)
		pd.fillIndent(parentIndex, i)
	}

	// Align the value of plan fields.
	pd.alignFields()

	for i, p := range pd.planInfos {
		if i > 0 {
			pd.buf.WriteByte(lineBreaker)
		}
		// This is for alignment.
		pd.buf.WriteByte(separator)
		pd.buf.WriteString(string(pd.indents[i]))
		for j := 0; j < len(p.fields); j++ {
			if j > 0 {
				pd.buf.WriteByte(separator)
			}
			pd.buf.WriteString(p.fields[j])
		}
	}
	return pd.buf.String(), nil
}

func (pd *planDecoder) addPlanHeader() {
	if len(pd.planInfos) == 0 {
		return
	}
	header := &planInfo{
		depth:  0,
		fields: []string{"id", "task", "estRows", "operator info", "actRows", "execution info", "memory", "disk"},
	}
	if len(pd.planInfos[0].fields) < len(header.fields) {
		// plan without runtime information.
		header.fields = header.fields[:len(pd.planInfos[0].fields)]
	}
	planInfos := make([]*planInfo, 0, len(pd.planInfos)+1)
	depths := make([]int, 0, len(pd.planInfos)+1)
	planInfos = append(planInfos, header)
	planInfos = append(planInfos, pd.planInfos...)
	depths = append(depths, header.depth)
	depths = append(depths, pd.depths...)
	pd.planInfos = planInfos
	pd.depths = depths
}

func (pd *planDecoder) initPlanTreeIndents() {
	pd.indents = pd.indents[:0]
	for i := 0; i < len(pd.depths); i++ {
		indent := make([]rune, 2*pd.depths[i])
		pd.indents = append(pd.indents, indent)
		if len(indent) == 0 {
			continue
		}
		for i := 0; i < len(indent)-2; i++ {
			indent[i] = ' '
		}
		indent[len(indent)-2] = TreeLastNode
		indent[len(indent)-1] = TreeNodeIdentifier
	}
}

func (pd *planDecoder) findParentIndex(childIndex int) int {
	pd.cacheParentIdent[pd.depths[childIndex]] = childIndex
	parentDepth := pd.depths[childIndex] - 1
	if parentIdx, ok := pd.cacheParentIdent[parentDepth]; ok {
		return parentIdx
	}
	for i := childIndex - 1; i > 0; i-- {
		if pd.depths[i] == parentDepth {
			pd.cacheParentIdent[pd.depths[i]] = i
			return i
		}
	}
	return 0
}

func (pd *planDecoder) fillIndent(parentIndex, childIndex int) {
	depth := pd.depths[childIndex]
	if depth == 0 {
		return
	}
	idx := depth*2 - 2
	for i := childIndex - 1; i > parentIndex; i-- {
		if pd.indents[i][idx] == TreeLastNode {
			pd.indents[i][idx] = TreeMiddleNode
			break
		}
		pd.indents[i][idx] = TreeBody
	}
}

func (pd *planDecoder) alignFields() {
	if len(pd.planInfos) == 0 {
		return
	}
	// Align fields length. Some plan may doesn't have runtime info, need append `` to align with other plan fields.
	maxLen := -1
	for _, p := range pd.planInfos {
		if len(p.fields) > maxLen {
			maxLen = len(p.fields)
		}
	}
	for _, p := range pd.planInfos {
		for len(p.fields) < maxLen {
			p.fields = append(p.fields, "")
		}
	}

	fieldsLen := len(pd.planInfos[0].fields)
	// Last field no need to align.
	fieldsLen--
	var buf []byte
	for colIdx := 0; colIdx < fieldsLen; colIdx++ {
		maxFieldLen := pd.getMaxFieldLength(colIdx)
		for rowIdx, p := range pd.planInfos {
			fillLen := maxFieldLen - pd.getPlanFieldLen(rowIdx, colIdx, p)
			for len(buf) < fillLen {
				buf = append(buf, ' ')
			}
			buf = buf[:fillLen]
			p.fields[colIdx] += string(buf)
		}
	}
}

func (pd *planDecoder) getMaxFieldLength(idx int) int {
	maxLength := -1
	for rowIdx, p := range pd.planInfos {
		l := pd.getPlanFieldLen(rowIdx, idx, p)
		if l > maxLength {
			maxLength = l
		}
	}
	return maxLength
}

func (pd *planDecoder) getPlanFieldLen(rowIdx, colIdx int, p *planInfo) int {
	if colIdx == 0 {
		return len(p.fields[0]) + len(pd.indents[rowIdx])
	}
	return len(p.fields[colIdx])
}

func decodePlanInfo(str string) (*planInfo, error) {
	values := strings.Split(str, separatorStr)
	if len(values) < 2 {
		return nil, nil
	}

	p := &planInfo{
		fields: make([]string, 0, len(values)-1),
	}
	for i, v := range values {
		switch i {
		// depth
		case 0:
			depth, err := strconv.Atoi(v)
			if err != nil {
				return nil, fmt.Errorf("decode plan: %v, depth: %v, error: %v", str, v, err)
			}
			p.depth = depth
		// plan ID
		case 1:
			ids := strings.Split(v, idSeparator)
			if len(ids) != 1 && len(ids) != 2 {
				return nil, fmt.Errorf("decode plan: %v error, invalid plan id: %v", str, v)
			}
			planID, err := strconv.Atoi(ids[0])
			if err != nil {
				return nil, fmt.Errorf("decode plan: %v, plan id: %v, error: %v", str, v, err)
			}
			if len(ids) == 1 {
				p.fields = append(p.fields, PhysicalIDToTypeString(planID))
			} else {
				p.fields = append(p.fields, PhysicalIDToTypeString(planID)+idSeparator+ids[1])
			}
		// task type
		case 2:
			task, err := decodeTaskType(v)
			if err != nil {
				return nil, fmt.Errorf("decode plan: %v, task type: %v, error: %v", str, v, err)
			}
			p.fields = append(p.fields, task)
		default:
			p.fields = append(p.fields, v)
		}
	}
	return p, nil
}

func decodeTaskType(str string) (string, error) {
	segs := strings.Split(str, idSeparator)
	if segs[0] == rootTaskType {
		return "root", nil
	}
	if len(segs) == 1 { // be compatible to `NormalizePlanNode`, which doesn't encode storeType in task field.
		return "cop", nil
	}
	storeType, err := strconv.Atoi(segs[1])
	if err != nil {
		return "", err
	}
	return "cop[" + ((StoreType)(storeType)).Name() + "]", nil
}

func decompress(str string) (string, error) {
	decodeBytes, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return "", err
	}

	bs, err := snappy.Decode(nil, decodeBytes)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

const (
	// TypeSel is the type of Selection.
	TypeSel = "Selection"
	// TypeSet is the type of Set.
	TypeSet = "Set"
	// TypeProj is the type of Projection.
	TypeProj = "Projection"
	// TypeAgg is the type of Aggregation.
	TypeAgg = "Aggregation"
	// TypeStreamAgg is the type of StreamAgg.
	TypeStreamAgg = "StreamAgg"
	// TypeHashAgg is the type of HashAgg.
	TypeHashAgg = "HashAgg"
	// TypeShow is the type of show.
	TypeShow = "Show"
	// TypeJoin is the type of Join.
	TypeJoin = "Join"
	// TypeUnion is the type of Union.
	TypeUnion = "Union"
	// TypePartitionUnion is the type of PartitionUnion
	TypePartitionUnion = "PartitionUnion"
	// TypeTableScan is the type of TableScan.
	TypeTableScan = "TableScan"
	// TypeMemTableScan is the type of TableScan.
	TypeMemTableScan = "MemTableScan"
	// TypeUnionScan is the type of UnionScan.
	TypeUnionScan = "UnionScan"
	// TypeIdxScan is the type of IndexScan.
	TypeIdxScan = "IndexScan"
	// TypeSort is the type of Sort.
	TypeSort = "Sort"
	// TypeTopN is the type of TopN.
	TypeTopN = "TopN"
	// TypeLimit is the type of Limit.
	TypeLimit = "Limit"
	// TypeHashJoin is the type of hash join.
	TypeHashJoin = "HashJoin"
	// TypeExchangeSender is the type of mpp exchanger sender.
	TypeExchangeSender = "ExchangeSender"
	// TypeExchangeReceiver is the type of mpp exchanger receiver.
	TypeExchangeReceiver = "ExchangeReceiver"
	// TypeMergeJoin is the type of merge join.
	TypeMergeJoin = "MergeJoin"
	// TypeIndexJoin is the type of index look up join.
	TypeIndexJoin = "IndexJoin"
	// TypeIndexMergeJoin is the type of index look up merge join.
	TypeIndexMergeJoin = "IndexMergeJoin"
	// TypeIndexHashJoin is the type of index nested loop hash join.
	TypeIndexHashJoin = "IndexHashJoin"
	// TypeApply is the type of Apply.
	TypeApply = "Apply"
	// TypeMaxOneRow is the type of MaxOneRow.
	TypeMaxOneRow = "MaxOneRow"
	// TypeExists is the type of Exists.
	TypeExists = "Exists"
	// TypeDual is the type of TableDual.
	TypeDual = "TableDual"
	// TypeLock is the type of SelectLock.
	TypeLock = "SelectLock"
	// TypeInsert is the type of Insert
	TypeInsert = "Insert"
	// TypeUpdate is the type of Update.
	TypeUpdate = "Update"
	// TypeDelete is the type of Delete.
	TypeDelete = "Delete"
	// TypeIndexLookUp is the type of IndexLookUp.
	TypeIndexLookUp = "IndexLookUp"
	// TypeTableReader is the type of TableReader.
	TypeTableReader = "TableReader"
	// TypeIndexReader is the type of IndexReader.
	TypeIndexReader = "IndexReader"
	// TypeWindow is the type of Window.
	TypeWindow = "Window"
	// TypeShuffle is the type of Shuffle.
	TypeShuffle = "Shuffle"
	// TypeShuffleReceiver is the type of Shuffle.
	TypeShuffleReceiver = "ShuffleReceiver"
	// TypeTiKVSingleGather is the type of TiKVSingleGather.
	TypeTiKVSingleGather = "TiKVSingleGather"
	// TypeIndexMerge is the type of IndexMergeReader
	TypeIndexMerge = "IndexMerge"
	// TypePointGet is the type of PointGetPlan.
	TypePointGet = "Point_Get"
	// TypeShowDDLJobs is the type of show ddl jobs.
	TypeShowDDLJobs = "ShowDDLJobs"
	// TypeBatchPointGet is the type of BatchPointGetPlan.
	TypeBatchPointGet = "Batch_Point_Get"
	// TypeClusterMemTableReader is the type of TableReader.
	TypeClusterMemTableReader = "ClusterMemTableReader"
	// TypeDataSource is the type of DataSource.
	TypeDataSource = "DataSource"
	// TypeLoadData is the type of LoadData.
	TypeLoadData = "LoadData"
	// TypeTableSample is the type of TableSample.
	TypeTableSample = "TableSample"
	// TypeTableFullScan is the type of TableFullScan.
	TypeTableFullScan = "TableFullScan"
	// TypeTableRangeScan is the type of TableRangeScan.
	TypeTableRangeScan = "TableRangeScan"
	// TypeTableRowIDScan is the type of TableRowIDScan.
	TypeTableRowIDScan = "TableRowIDScan"
	// TypeIndexFullScan is the type of IndexFullScan.
	TypeIndexFullScan = "IndexFullScan"
	// TypeIndexRangeScan is the type of IndexRangeScan.
	TypeIndexRangeScan = "IndexRangeScan"
	// TypeCTETable is the type of TypeCTETable.
	TypeCTETable = "CTETable"
	// TypeCTE is the type of CTEFullScan.
	TypeCTE = "CTEFullScan"
	// TypeCTEDefinition is the type of CTE definition
	TypeCTEDefinition = "CTE"
)

// plan id.
// Attention: for compatibility of encode/decode plan, The plan id shouldn't be changed.
const (
	typeSelID                 int = 1
	typeSetID                 int = 2
	typeProjID                int = 3
	typeAggID                 int = 4
	typeStreamAggID           int = 5
	typeHashAggID             int = 6
	typeShowID                int = 7
	typeJoinID                int = 8
	typeUnionID               int = 9
	typeTableScanID           int = 10
	typeMemTableScanID        int = 11
	typeUnionScanID           int = 12
	typeIdxScanID             int = 13
	typeSortID                int = 14
	typeTopNID                int = 15
	typeLimitID               int = 16
	typeHashJoinID            int = 17
	typeMergeJoinID           int = 18
	typeIndexJoinID           int = 19
	typeIndexMergeJoinID      int = 20
	typeIndexHashJoinID       int = 21
	typeApplyID               int = 22
	typeMaxOneRowID           int = 23
	typeExistsID              int = 24
	typeDualID                int = 25
	typeLockID                int = 26
	typeInsertID              int = 27
	typeUpdateID              int = 28
	typeDeleteID              int = 29
	typeIndexLookUpID         int = 30
	typeTableReaderID         int = 31
	typeIndexReaderID         int = 32
	typeWindowID              int = 33
	typeTiKVSingleGatherID    int = 34
	typeIndexMergeID          int = 35
	typePointGet              int = 36
	typeShowDDLJobs           int = 37
	typeBatchPointGet         int = 38
	typeClusterMemTableReader int = 39
	typeDataSourceID          int = 40
	typeLoadDataID            int = 41
	typeTableSampleID         int = 42
	typeTableFullScan         int = 43
	typeTableRangeScan        int = 44
	typeTableRowIDScan        int = 45
	typeIndexFullScan         int = 46
	typeIndexRangeScan        int = 47
	typeExchangeReceiver      int = 48
	typeExchangeSender        int = 49
	typeCTE                   int = 50
	typeCTEDefinition         int = 51
	typeCTETable              int = 52
)

// PhysicalIDToTypeString converts the plan id to plan type string.
func PhysicalIDToTypeString(id int) string {
	switch id {
	case typeSelID:
		return TypeSel
	case typeSetID:
		return TypeSet
	case typeProjID:
		return TypeProj
	case typeAggID:
		return TypeAgg
	case typeStreamAggID:
		return TypeStreamAgg
	case typeHashAggID:
		return TypeHashAgg
	case typeShowID:
		return TypeShow
	case typeJoinID:
		return TypeJoin
	case typeUnionID:
		return TypeUnion
	case typeTableScanID:
		return TypeTableScan
	case typeMemTableScanID:
		return TypeMemTableScan
	case typeUnionScanID:
		return TypeUnionScan
	case typeIdxScanID:
		return TypeIdxScan
	case typeSortID:
		return TypeSort
	case typeTopNID:
		return TypeTopN
	case typeLimitID:
		return TypeLimit
	case typeHashJoinID:
		return TypeHashJoin
	case typeMergeJoinID:
		return TypeMergeJoin
	case typeIndexJoinID:
		return TypeIndexJoin
	case typeIndexMergeJoinID:
		return TypeIndexMergeJoin
	case typeIndexHashJoinID:
		return TypeIndexHashJoin
	case typeApplyID:
		return TypeApply
	case typeMaxOneRowID:
		return TypeMaxOneRow
	case typeExistsID:
		return TypeExists
	case typeDualID:
		return TypeDual
	case typeLockID:
		return TypeLock
	case typeInsertID:
		return TypeInsert
	case typeUpdateID:
		return TypeUpdate
	case typeDeleteID:
		return TypeDelete
	case typeIndexLookUpID:
		return TypeIndexLookUp
	case typeTableReaderID:
		return TypeTableReader
	case typeIndexReaderID:
		return TypeIndexReader
	case typeWindowID:
		return TypeWindow
	case typeTiKVSingleGatherID:
		return TypeTiKVSingleGather
	case typeIndexMergeID:
		return TypeIndexMerge
	case typePointGet:
		return TypePointGet
	case typeShowDDLJobs:
		return TypeShowDDLJobs
	case typeBatchPointGet:
		return TypeBatchPointGet
	case typeClusterMemTableReader:
		return TypeClusterMemTableReader
	case typeLoadDataID:
		return TypeLoadData
	case typeTableSampleID:
		return TypeTableSample
	case typeTableFullScan:
		return TypeTableFullScan
	case typeTableRangeScan:
		return TypeTableRangeScan
	case typeTableRowIDScan:
		return TypeTableRowIDScan
	case typeIndexFullScan:
		return TypeIndexFullScan
	case typeIndexRangeScan:
		return TypeIndexRangeScan
	case typeExchangeReceiver:
		return TypeExchangeReceiver
	case typeExchangeSender:
		return TypeExchangeSender
	case typeCTE:
		return TypeCTE
	case typeCTEDefinition:
		return TypeCTEDefinition
	case typeCTETable:
		return TypeCTETable
	}

	// Should never reach here.
	return "UnknownPlanID" + strconv.Itoa(id)
}

type StoreType uint8

const (
	// TiKV means the type of a store is Ti
	TiKV StoreType = iota
	// TiFlash means the type of a store is TiFlash.
	TiFlash
	// TiDB means the type of a store is TiDB.
	TiDB
	// UnSpecified means the store type is unknown
	UnSpecified = 255
)

// Name returns the name of store type.
func (t StoreType) Name() string {
	if t == TiFlash {
		return "tiflash"
	} else if t == TiDB {
		return "tidb"
	} else if t == TiKV {
		return "tikv"
	}
	return "unspecified"
}

const (
	// TreeBody indicates the current operator sub-tree is not finished, still
	// has child operators to be attached on.
	TreeBody = '│'
	// TreeMiddleNode indicates this operator is not the last child of the
	// current sub-tree rooted by its parent.
	TreeMiddleNode = '├'
	// TreeLastNode indicates this operator is the last child of the current
	// sub-tree rooted by its parent.
	TreeLastNode = '└'
	// TreeGap is used to represent the gap between the branches of the tree.
	TreeGap = ' '
	// TreeNodeIdentifier is used to replace the treeGap once we need to attach
	// a node to a sub-tree.
	TreeNodeIdentifier = '─'
)
