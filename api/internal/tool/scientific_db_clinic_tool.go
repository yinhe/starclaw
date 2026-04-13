package tool

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"gorm.io/gorm"
)

type ScientificDBClinicTool struct {
	db *gorm.DB
}

func NewScientificDBClinicTool(db *gorm.DB) *ScientificDBClinicTool {
	return &ScientificDBClinicTool{db: db}
}

func (t *ScientificDBClinicTool) Execute(userID string, params map[string]interface{}) (string, error) {
	action, _ := params["action"].(string)
	switch action {
	case "pubmed_query_builder":
		return t.pubmedQueryBuilder(params)
	case "clinical_trials_router":
		return t.clinicalTrialsRouter(params)
	case "guideline_router":
		return t.guidelineRouter(params)
	case "gene_disease_router":
		return t.geneDiseaseRouter(params)
	case "variant_database_router":
		return t.variantDatabaseRouter(params)
	case "protein_structure_router":
		return t.proteinStructureRouter(params)
	case "drug_target_router":
		return t.drugTargetRouter(params)
	case "pathway_router":
		return t.pathwayRouter(params)
	case "expression_router":
		return t.expressionRouter(params)
	case "sequence_similarity_router":
		return t.sequenceSimilarityRouter(params)
	case "omics_dataset_router":
		return t.omicsDatasetRouter(params)
	case "sequence_record_router":
		return t.sequenceRecordRouter(params)
	case "domain_family_router":
		return t.domainFamilyRouter(params)
	case "transcript_router":
		return t.transcriptRouter(params)
	case "genome_browser_router":
		return t.genomeBrowserRouter(params)
	case "ortholog_router":
		return t.orthologRouter(params)
	case "protein_interaction_router":
		return t.proteinInteractionRouter(params)
	case "phenotype_router":
		return t.phenotypeRouter(params)
	case "model_organism_router":
		return t.modelOrganismRouter(params)
	case "cell_line_router":
		return t.cellLineRouter(params)
	case "epigenomics_router":
		return t.epigenomicsRouter(params)
	case "single_cell_router":
		return t.singleCellRouter(params)
	case "proteomics_router":
		return t.proteomicsRouter(params)
	case "metabolomics_router":
		return t.metabolomicsRouter(params)
	case "microbiome_router":
		return t.microbiomeRouter(params)
	case "pharmacogenomics_router":
		return t.pharmacogenomicsRouter(params)
	case "immunology_router":
		return t.immunologyRouter(params)
	case "toxicogenomics_router":
		return t.toxicogenomicsRouter(params)
	case "biobank_router":
		return t.biobankRouter(params)
	default:
		return "", fmt.Errorf("unknown scientific_db_clinic action: %s", action)
	}
}

func (t *ScientificDBClinicTool) pubmedQueryBuilder(params map[string]interface{}) (string, error) {
	disease, _ := params["disease"].(string)
	population, _ := params["population"].(string)
	intervention, _ := params["intervention"].(string)
	outcome, _ := params["outcome"].(string)
	studyTypes := parseScientificDBList(params["study_types"])
	keywords := parseScientificDBList(params["keywords"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{disease, population, intervention, outcome}, keywords...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one literature search term is required")
	}

	segments := []string{}
	for _, term := range coreTerms {
		segments = append(segments, fmt.Sprintf("\"%s\"", strings.TrimSpace(term)))
	}

	filters := []string{}
	studyLookup := map[string]string{
		"meta-analysis":     "meta-analysis[Publication Type]",
		"systematicreview":  "systematic review[Title/Abstract]",
		"randomized":        "randomized controlled trial[Publication Type]",
		"rct":               "randomized controlled trial[Publication Type]",
		"cohort":            "cohort[Title/Abstract]",
		"casereport":        "case reports[Publication Type]",
		"guideline":         "guideline[Publication Type]",
		"practiceguideline": "practice guideline[Publication Type]",
	}
	for _, studyType := range studyTypes {
		normalized := normalizeScientificDBTerm(studyType)
		if mapped, ok := studyLookup[normalized]; ok {
			filters = append(filters, mapped)
		} else if strings.TrimSpace(studyType) != "" {
			filters = append(filters, fmt.Sprintf("\"%s\"", strings.TrimSpace(studyType)))
		}
	}

	query := strings.Join(segments, " AND ")
	if len(filters) > 0 {
		query = query + " AND (" + strings.Join(uniqueSortedScientificDBStrings(filters), " OR ") + ")"
	}

	searchURL := "https://pubmed.ncbi.nlm.nih.gov/?term=" + url.QueryEscape(query)
	secondary := []map[string]interface{}{
		{"source": "PubMed", "url": searchURL, "focus": "临床原始研究与综述"},
		{"source": "Google Scholar", "url": "https://scholar.google.com/scholar?q=" + url.QueryEscape(strings.Join(coreTerms, " ")), "focus": "补充引文追踪与灰色文献"},
	}
	if containsAnyNormalizedScientificDB(studyTypes, "systematic review", "meta-analysis", "guideline") {
		secondary = append(secondary, map[string]interface{}{"source": "Cochrane Library", "url": "https://www.cochranelibrary.com/search?searchRow.searchOptions.searchProducts=all&searchRow.searchOptions.doBooleanSearch=true&searchRow.searchOptions.searchText=" + url.QueryEscape(strings.Join(coreTerms, " ")), "focus": "系统综述与证据综合"})
	}

	return jsonStr(map[string]interface{}{
		"panel":               "pubmed_query_builder",
		"query":               query,
		"pubmed_url":          searchURL,
		"recommended_sources": secondary,
		"search_terms":        coreTerms,
		"study_type_filters":  filters,
		"followup":            "建议先用该检索式查看题录，再结合摘要、发表时间和研究类型筛选高质量证据。",
	}), nil
}

func (t *ScientificDBClinicTool) clinicalTrialsRouter(params map[string]interface{}) (string, error) {
	condition, _ := params["condition"].(string)
	intervention, _ := params["intervention"].(string)
	ageGroup, _ := params["age_group"].(string)
	recruitingStatus, _ := params["recruiting_status"].(string)
	region, _ := params["region"].(string)
	biomarkers := parseScientificDBList(params["biomarkers"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{condition, intervention, ageGroup, recruitingStatus, region}, biomarkers...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one clinical trial search term is required")
	}

	clinicalTerms := []string{}
	if strings.TrimSpace(condition) != "" {
		clinicalTerms = append(clinicalTerms, condition)
	}
	if strings.TrimSpace(intervention) != "" {
		clinicalTerms = append(clinicalTerms, intervention)
	}
	clinicalTerms = append(clinicalTerms, biomarkers...)
	if strings.TrimSpace(ageGroup) != "" {
		clinicalTerms = append(clinicalTerms, ageGroup)
	}
	query := strings.Join(uniqueSortedScientificDBStrings(clinicalTerms), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	statusNote := "未指定招募状态"
	if strings.TrimSpace(recruitingStatus) != "" {
		statusNote = strings.TrimSpace(recruitingStatus)
	}

	sources := []map[string]interface{}{
		{"source": "ClinicalTrials.gov", "url": "https://clinicaltrials.gov/search?term=" + url.QueryEscape(query), "focus": "全球主要注册试验", "status": statusNote},
		{"source": "WHO ICTRP", "url": "https://trialsearch.who.int/?q=" + url.QueryEscape(query), "focus": "国际多注册平台检索", "status": statusNote},
	}
	if containsAnyNormalizedScientificDB([]string{region}, "europe", "eu", "欧洲") {
		sources = append(sources, map[string]interface{}{"source": "EU Clinical Trials Register", "url": "https://www.clinicaltrialsregister.eu/ctr-search/search?query=" + url.QueryEscape(query), "focus": "欧洲注册试验"})
	}
	if containsAnyNormalizedScientificDB([]string{region}, "china", "cn", "中国") {
		sources = append(sources, map[string]interface{}{"source": "中国临床试验注册中心", "url": "http://www.chictr.org.cn/searchproj.aspx?keyword=" + url.QueryEscape(query), "focus": "中国注册试验"})
	}

	return jsonStr(map[string]interface{}{
		"panel":               "clinical_trials_router",
		"query":               query,
		"recommended_sources": sources,
		"condition":           strings.TrimSpace(condition),
		"intervention":        strings.TrimSpace(intervention),
		"age_group":           strings.TrimSpace(ageGroup),
		"region":              strings.TrimSpace(region),
		"followup":            "建议优先筛选正在招募、年龄匹配且结局指标相关的试验，并复核纳入排除标准。",
	}), nil
}

func (t *ScientificDBClinicTool) guidelineRouter(params map[string]interface{}) (string, error) {
	topic, _ := params["topic"].(string)
	specialty, _ := params["specialty"].(string)
	population, _ := params["population"].(string)
	region, _ := params["region"].(string)
	keywords := parseScientificDBList(params["keywords"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{topic, specialty, population, region}, keywords...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one guideline search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{topic, population}, keywords...)), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	sourceSites := map[string]string{}
	for _, source := range selectGuidelineSources(specialty, topic, region) {
		sourceSites[source] = "https://duckduckgo.com/?q=" + url.QueryEscape("site:"+source+" "+query+" guideline")
	}

	recommended := []map[string]interface{}{}
	keys := make([]string, 0, len(sourceSites))
	for key := range sourceSites {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		recommended = append(recommended, map[string]interface{}{"source": key, "search_url": sourceSites[key]})
	}

	return jsonStr(map[string]interface{}{
		"panel":               "guideline_router",
		"query":               query,
		"recommended_sources": recommended,
		"specialty":           strings.TrimSpace(specialty),
		"population":          strings.TrimSpace(population),
		"region":              strings.TrimSpace(region),
		"followup":            "建议优先查看近年指南、更新说明与证据分级，再结合本地实践和患者特征综合使用。",
	}), nil
}

func (t *ScientificDBClinicTool) geneDiseaseRouter(params map[string]interface{}) (string, error) {
	gene, _ := params["gene"].(string)
	disease, _ := params["disease"].(string)
	variant, _ := params["variant"].(string)
	inheritance, _ := params["inheritance"].(string)
	phenotypes := parseScientificDBList(params["phenotypes"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{gene, disease, variant, inheritance}, phenotypes...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one gene or disease database search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{gene, disease, variant}, phenotypes...)), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	sources := []map[string]interface{}{
		{"source": "OMIM", "url": "https://omim.org/search?index=entry&search=" + url.QueryEscape(query), "focus": "基因-疾病关联与遗传模式"},
		{"source": "GeneReviews", "url": "https://www.ncbi.nlm.nih.gov/books/?term=" + url.QueryEscape(query), "focus": "临床遗传学综述"},
		{"source": "ClinVar", "url": "https://www.ncbi.nlm.nih.gov/clinvar/?term=" + url.QueryEscape(query), "focus": "变异临床意义"},
		{"source": "Orphanet", "url": "https://www.orpha.net/en/disease/search?query=" + url.QueryEscape(query), "focus": "罕见病信息与编码"},
		{"source": "MedGen", "url": "https://www.ncbi.nlm.nih.gov/medgen/?term=" + url.QueryEscape(query), "focus": "疾病概念与同义词扩展"},
	}

	return jsonStr(map[string]interface{}{
		"panel":               "gene_disease_router",
		"query":               query,
		"gene":                strings.TrimSpace(gene),
		"disease":             strings.TrimSpace(disease),
		"variant":             strings.TrimSpace(variant),
		"inheritance":         strings.TrimSpace(inheritance),
		"recommended_sources": sources,
		"phenotypes":          phenotypes,
		"followup":            "建议先统一基因/疾病命名，再交叉核对 OMIM、ClinVar、GeneReviews 与 Orphanet 的证据一致性。",
	}), nil
}

func (t *ScientificDBClinicTool) variantDatabaseRouter(params map[string]interface{}) (string, error) {
	gene, _ := params["gene"].(string)
	variant, _ := params["variant"].(string)
	disease, _ := params["disease"].(string)
	genomeBuild, _ := params["genome_build"].(string)
	phenotypes := parseScientificDBList(params["phenotypes"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{gene, variant, disease, genomeBuild}, phenotypes...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one variant database search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{gene, variant, disease}, phenotypes...)), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	sources := []map[string]interface{}{
		{"source": "ClinVar", "url": "https://www.ncbi.nlm.nih.gov/clinvar/?term=" + url.QueryEscape(query), "focus": "临床意义与提交证据"},
		{"source": "dbSNP", "url": "https://www.ncbi.nlm.nih.gov/snp/?term=" + url.QueryEscape(query), "focus": "rs 编号与变异基础信息"},
		{"source": "gnomAD", "url": "https://gnomad.broadinstitute.org/search?query=" + url.QueryEscape(query), "focus": "群体频率与约束信息"},
		{"source": "LOVD", "url": "https://databases.lovd.nl/shared/variants?search_variantDB=" + url.QueryEscape(query), "focus": "位点与基因特异数据库"},
	}
	if containsAnyNormalizedScientificDB([]string{disease}, "cancer", "tumor", "肿瘤", "癌") {
		sources = append(sources, map[string]interface{}{"source": "CIViC", "url": "https://civicdb.org/search/variants?query=" + url.QueryEscape(query), "focus": "肿瘤变异解释与证据分级"})
	}

	return jsonStr(map[string]interface{}{
		"panel":               "variant_database_router",
		"query":               query,
		"gene":                strings.TrimSpace(gene),
		"variant":             strings.TrimSpace(variant),
		"disease":             strings.TrimSpace(disease),
		"genome_build":        strings.TrimSpace(genomeBuild),
		"phenotypes":          phenotypes,
		"recommended_sources": sources,
		"followup":            "建议统一 HGVS/rsID/基因命名后，再交叉核对 ClinVar、gnomAD 与疾病特异数据库。",
	}), nil
}

func (t *ScientificDBClinicTool) proteinStructureRouter(params map[string]interface{}) (string, error) {
	gene, _ := params["gene"].(string)
	protein, _ := params["protein"].(string)
	organism, _ := params["organism"].(string)
	mutation, _ := params["mutation"].(string)
	keywords := parseScientificDBList(params["keywords"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{gene, protein, organism, mutation}, keywords...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one protein or structure search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{gene, protein, mutation}, keywords...)), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	sources := []map[string]interface{}{
		{"source": "AlphaFold DB", "url": "https://alphafold.ebi.ac.uk/search/text/" + url.QueryEscape(query), "focus": "预测蛋白结构"},
		{"source": "RCSB PDB", "url": "https://www.rcsb.org/search?query=" + url.QueryEscape(query), "focus": "实验结构与复合物"},
		{"source": "UniProt", "url": "https://www.uniprot.org/uniprotkb?query=" + url.QueryEscape(query), "focus": "蛋白功能、同工型与注释"},
		{"source": "PDBe-KB", "url": "https://www.ebi.ac.uk/pdbe/pdbe-kb/search?keyword=" + url.QueryEscape(query), "focus": "结构功能知识整合"},
		{"source": "InterPro", "url": "https://www.ebi.ac.uk/interpro/search/text/" + url.QueryEscape(query), "focus": "结构域与家族注释"},
	}

	return jsonStr(map[string]interface{}{
		"panel":               "protein_structure_router",
		"query":               query,
		"gene":                strings.TrimSpace(gene),
		"protein":             strings.TrimSpace(protein),
		"organism":            strings.TrimSpace(organism),
		"mutation":            strings.TrimSpace(mutation),
		"recommended_sources": sources,
		"followup":            "建议先统一 UniProt/基因名，再根据是否已有实验结构决定优先查 PDB 还是 AlphaFold DB。",
	}), nil
}

func (t *ScientificDBClinicTool) drugTargetRouter(params map[string]interface{}) (string, error) {
	drug, _ := params["drug"].(string)
	target, _ := params["target"].(string)
	disease, _ := params["disease"].(string)
	mechanism, _ := params["mechanism"].(string)
	biomarkers := parseScientificDBList(params["biomarkers"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{drug, target, disease, mechanism}, biomarkers...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one drug or target search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{drug, target, disease, mechanism}, biomarkers...)), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	sources := []map[string]interface{}{
		{"source": "DrugBank", "url": "https://go.drugbank.com/unearth/q?searcher=drugs&query=" + url.QueryEscape(query), "focus": "药物信息、靶点与相互作用"},
		{"source": "ChEMBL", "url": "https://www.ebi.ac.uk/chembl/g/#search_results/all/query=" + url.QueryEscape(query), "focus": "活性数据与药化证据"},
		{"source": "BindingDB", "url": "https://www.bindingdb.org/rwd/bind/search?searchType=quick&query=" + url.QueryEscape(query), "focus": "结合活性与配体-靶点数据"},
		{"source": "DGIdb", "url": "https://www.dgidb.org/search_interactions?search_terms=" + url.QueryEscape(query), "focus": "药物-基因相互作用"},
		{"source": "DailyMed", "url": "https://dailymed.nlm.nih.gov/dailymed/search.cfm?query=" + url.QueryEscape(query), "focus": "说明书与标签信息"},
	}
	if len(biomarkers) > 0 {
		sources = append(sources, map[string]interface{}{"source": "PharmGKB", "url": "https://www.pharmgkb.org/search?query=" + url.QueryEscape(query), "focus": "药物基因组学与生物标志物"})
	}

	return jsonStr(map[string]interface{}{
		"panel":               "drug_target_router",
		"query":               query,
		"drug":                strings.TrimSpace(drug),
		"target":              strings.TrimSpace(target),
		"disease":             strings.TrimSpace(disease),
		"mechanism":           strings.TrimSpace(mechanism),
		"biomarkers":          biomarkers,
		"recommended_sources": sources,
		"followup":            "建议先统一药物通用名、靶点基因名和机制关键词，再交叉核对 DrugBank、ChEMBL 与临床标签来源。",
	}), nil
}

func (t *ScientificDBClinicTool) pathwayRouter(params map[string]interface{}) (string, error) {
	gene, _ := params["gene"].(string)
	pathway, _ := params["pathway"].(string)
	disease, _ := params["disease"].(string)
	organism, _ := params["organism"].(string)
	keywords := parseScientificDBList(params["keywords"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{gene, pathway, disease, organism}, keywords...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one pathway database search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{gene, pathway, disease}, keywords...)), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	sources := []map[string]interface{}{
		{"source": "KEGG Pathway", "url": "https://www.genome.jp/dbget-bin/www_bfind_sub?mode=bfind&max_hit=1000&locale=en&serv=kegg&dbkey=pathway&keywords=" + url.QueryEscape(query), "focus": "经典代谢与信号通路"},
		{"source": "Reactome", "url": "https://reactome.org/content/query?q=" + url.QueryEscape(query), "focus": "机制化通路与事件层级"},
		{"source": "WikiPathways", "url": "https://www.wikipathways.org/index.php/Special:SearchPathway?query=" + url.QueryEscape(query), "focus": "社区维护通路图谱"},
		{"source": "MSigDB", "url": "https://www.gsea-msigdb.org/gsea/msigdb/search.jsp?q=" + url.QueryEscape(query), "focus": "基因集富集与签名检索"},
		{"source": "Pathway Commons", "url": "https://www.pathwaycommons.org/pc2/search?q=" + url.QueryEscape(query), "focus": "跨数据库整合通路关系"},
	}
	if containsAnyNormalizedScientificDB([]string{disease, pathway}, "cancer", "tumor", "oncology", "肿瘤", "癌") {
		sources = append(sources, map[string]interface{}{"source": "cBioPortal", "url": "https://www.cbioportal.org/results/pathways?gene_list=" + url.QueryEscape(strings.Join(uniqueSortedScientificDBStrings(append([]string{gene}, keywords...)), ",")), "focus": "肿瘤队列中的通路扰动"})
	}

	return jsonStr(map[string]interface{}{
		"panel":               "pathway_router",
		"query":               query,
		"gene":                strings.TrimSpace(gene),
		"pathway":             strings.TrimSpace(pathway),
		"disease":             strings.TrimSpace(disease),
		"organism":            strings.TrimSpace(organism),
		"keywords":            keywords,
		"recommended_sources": sources,
		"followup":            "建议先统一基因名和通路关键词，再按 KEGG/Reactome/WikiPathways 的顺序交叉核对定义、成员基因和上下游事件。",
	}), nil
}

func (t *ScientificDBClinicTool) expressionRouter(params map[string]interface{}) (string, error) {
	gene, _ := params["gene"].(string)
	disease, _ := params["disease"].(string)
	tissue, _ := params["tissue"].(string)
	organism, _ := params["organism"].(string)
	cellType, _ := params["cell_type"].(string)
	keywords := parseScientificDBList(params["keywords"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{gene, disease, tissue, organism, cellType}, keywords...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one expression database search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{gene, disease, tissue, cellType}, keywords...)), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	sources := []map[string]interface{}{
		{"source": "NCBI GEO", "url": "https://www.ncbi.nlm.nih.gov/gds/?term=" + url.QueryEscape(query), "focus": "转录组与表达矩阵数据集"},
		{"source": "Expression Atlas", "url": "https://www.ebi.ac.uk/gxa/search?query=" + url.QueryEscape(query), "focus": "基因、组织和疾病表达概览"},
		{"source": "Human Protein Atlas", "url": "https://www.proteinatlas.org/search/" + url.QueryEscape(query), "focus": "组织、细胞与蛋白表达图谱"},
		{"source": "ArrayExpress", "url": "https://www.ebi.ac.uk/biostudies/arrayexpress/studies?query=" + url.QueryEscape(query), "focus": "公共表达实验归档"},
	}
	if containsAnyNormalizedScientificDB([]string{organism, tissue}, "human", "homo sapiens", "人") {
		sources = append(sources, map[string]interface{}{"source": "GTEx", "url": "https://gtexportal.org/home/gene/" + url.QueryEscape(strings.TrimSpace(gene)), "focus": "人体组织特异表达"})
	}
	if strings.TrimSpace(cellType) != "" || containsAnyNormalizedScientificDB(keywords, "single cell", "single-cell", "单细胞") {
		sources = append(sources, map[string]interface{}{"source": "Single Cell Expression Atlas", "url": "https://www.ebi.ac.uk/gxa/sc/home?query=" + url.QueryEscape(query), "focus": "单细胞表达与细胞类型分层"})
	}

	return jsonStr(map[string]interface{}{
		"panel":               "expression_router",
		"query":               query,
		"gene":                strings.TrimSpace(gene),
		"disease":             strings.TrimSpace(disease),
		"tissue":              strings.TrimSpace(tissue),
		"organism":            strings.TrimSpace(organism),
		"cell_type":           strings.TrimSpace(cellType),
		"keywords":            keywords,
		"recommended_sources": sources,
		"followup":            "建议先确认基因符号、组织与细胞类型命名，再区分 bulk expression、单细胞表达和蛋白层图谱分别检索。",
	}), nil
}

func (t *ScientificDBClinicTool) sequenceSimilarityRouter(params map[string]interface{}) (string, error) {
	sequence, _ := params["sequence"].(string)
	accession, _ := params["accession"].(string)
	gene, _ := params["gene"].(string)
	organism, _ := params["organism"].(string)
	typeHint, _ := params["sequence_type_hint"].(string)
	databaseScope, _ := params["database_scope"].(string)
	keywords := parseScientificDBList(params["keywords"])

	cleanedSequence := extractScientificDBSequence(sequence)
	coreTerms := uniqueSortedScientificDBStrings(append([]string{accession, gene, organism, typeHint, databaseScope}, keywords...))
	if len(coreTerms) == 0 && cleanedSequence == "" {
		return "", fmt.Errorf("at least one sequence similarity search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{accession, gene, organism, databaseScope}, keywords...)), " ")
	if query == "" {
		if cleanedSequence != "" {
			query = fmt.Sprintf("sequence length %d", len(cleanedSequence))
		} else {
			query = strings.Join(coreTerms, " ")
		}
	}

	sequenceType := inferScientificDBSequenceType(cleanedSequence, typeHint)
	sources := []map[string]interface{}{}
	if sequenceType == "protein" {
		sources = append(sources,
			map[string]interface{}{"source": "NCBI BLASTP", "url": "https://blast.ncbi.nlm.nih.gov/Blast.cgi?PROGRAM=blastp&PAGE_TYPE=BlastSearch", "focus": "蛋白相似性检索与同源蛋白比对"},
			map[string]interface{}{"source": "UniProt BLAST", "url": "https://www.uniprot.org/blast", "focus": "UniProt 蛋白库快速同源检索"},
			map[string]interface{}{"source": "EMBL-EBI FASTA", "url": "https://www.ebi.ac.uk/jdispatcher/sss/fasta", "focus": "蛋白或核酸 FASTA 相似性检索"},
			map[string]interface{}{"source": "HMMER", "url": "https://www.ebi.ac.uk/Tools/hmmer/", "focus": "蛋白家族与保守结构域相似性"},
		)
	} else {
		sources = append(sources,
			map[string]interface{}{"source": "NCBI BLASTN", "url": "https://blast.ncbi.nlm.nih.gov/Blast.cgi?PROGRAM=blastn&PAGE_TYPE=BlastSearch", "focus": "核酸序列同源检索"},
			map[string]interface{}{"source": "EMBL-EBI FASTA", "url": "https://www.ebi.ac.uk/jdispatcher/sss/fasta", "focus": "核酸或蛋白 FASTA 相似性检索"},
			map[string]interface{}{"source": "UCSC BLAT", "url": "https://genome.ucsc.edu/cgi-bin/hgBlat", "focus": "基因组快速定位与近似匹配"},
			map[string]interface{}{"source": "Ensembl BLAST/BLAT", "url": "https://www.ensembl.org/Multi/Tools/Blast", "focus": "参考基因组与转录本层检索"},
		)
	}
	if strings.TrimSpace(accession) != "" {
		lookupSource := "NCBI Nucleotide"
		if sequenceType == "protein" {
			lookupSource = "NCBI Protein"
		}
		sources = append(sources, map[string]interface{}{"source": lookupSource, "url": "https://www.ncbi.nlm.nih.gov/search/all/?term=" + url.QueryEscape(strings.TrimSpace(accession)), "focus": "按 accession 先确认原始条目与版本"})
	}

	return jsonStr(map[string]interface{}{
		"panel":               "sequence_similarity_router",
		"query":               query,
		"accession":           strings.TrimSpace(accession),
		"gene":                strings.TrimSpace(gene),
		"organism":            strings.TrimSpace(organism),
		"sequence_type":       sequenceType,
		"sequence_length":     len(cleanedSequence),
		"sequence_preview":    cleanedSequence[:minScientificDBInt(len(cleanedSequence), 24)],
		"database_scope":      strings.TrimSpace(databaseScope),
		"keywords":            keywords,
		"recommended_sources": sources,
		"followup":            "建议先确认 accession 或序列类型，再决定走 BLAST、BLAT 还是蛋白家族层相似性检索。",
	}), nil
}

func (t *ScientificDBClinicTool) omicsDatasetRouter(params map[string]interface{}) (string, error) {
	gene, _ := params["gene"].(string)
	disease, _ := params["disease"].(string)
	tissue, _ := params["tissue"].(string)
	organism, _ := params["organism"].(string)
	omicsType, _ := params["omics_type"].(string)
	keywords := parseScientificDBList(params["keywords"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{gene, disease, tissue, organism, omicsType}, keywords...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one omics dataset search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{gene, disease, tissue, omicsType}, keywords...)), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	sources := []map[string]interface{}{
		{"source": "NCBI GEO", "url": "https://www.ncbi.nlm.nih.gov/gds/?term=" + url.QueryEscape(query), "focus": "转录组与功能基因组数据集"},
		{"source": "NCBI SRA", "url": "https://www.ncbi.nlm.nih.gov/sra/?term=" + url.QueryEscape(query), "focus": "原始测序 reads 与项目入口"},
		{"source": "BioStudies", "url": "https://www.ebi.ac.uk/biostudies/studies?query=" + url.QueryEscape(query), "focus": "多类型组学研究归档"},
		{"source": "BioProject", "url": "https://www.ncbi.nlm.nih.gov/bioproject/?term=" + url.QueryEscape(query), "focus": "项目级元数据与关联样本"},
	}
	if containsAnyNormalizedScientificDB([]string{omicsType}, "proteomics", "protein", "蛋白") {
		sources = append(sources, map[string]interface{}{"source": "PRIDE", "url": "https://www.ebi.ac.uk/pride/archive/simpleSearch?keyword=" + url.QueryEscape(query), "focus": "蛋白质组与质谱数据集"})
	}
	if containsAnyNormalizedScientificDB([]string{omicsType}, "metabolomics", "metabolome", "代谢") {
		sources = append(sources, map[string]interface{}{"source": "MetaboLights", "url": "https://www.ebi.ac.uk/metabolights/ws/studies/search?query=" + url.QueryEscape(query), "focus": "代谢组数据与实验条件"})
	}
	if containsAnyNormalizedScientificDB([]string{omicsType}, "epigenomics", "chip", "atac", "methyl", "表观") {
		sources = append(sources, map[string]interface{}{"source": "ENCODE", "url": "https://www.encodeproject.org/search/?searchTerm=" + url.QueryEscape(query), "focus": "表观组、调控组与功能元件数据"})
	}
	if containsAnyNormalizedScientificDB([]string{omicsType}, "single cell", "single-cell", "scrna", "单细胞") {
		sources = append(sources, map[string]interface{}{"source": "Single Cell Portal", "url": "https://singlecell.broadinstitute.org/single_cell/study?searchTerm=" + url.QueryEscape(query), "focus": "单细胞表达与细胞亚群研究"})
	}
	if containsAnyNormalizedScientificDB([]string{disease}, "cancer", "tumor", "oncology", "肿瘤", "癌") {
		sources = append(sources, map[string]interface{}{"source": "cBioPortal", "url": "https://www.cbioportal.org/results?gene_list=" + url.QueryEscape(strings.Join(uniqueSortedScientificDBStrings(append([]string{gene}, keywords...)), ",")), "focus": "肿瘤多组学队列与病例层数据入口"})
	}

	return jsonStr(map[string]interface{}{
		"panel":               "omics_dataset_router",
		"query":               query,
		"gene":                strings.TrimSpace(gene),
		"disease":             strings.TrimSpace(disease),
		"tissue":              strings.TrimSpace(tissue),
		"organism":            strings.TrimSpace(organism),
		"omics_type":          strings.TrimSpace(omicsType),
		"keywords":            keywords,
		"recommended_sources": sources,
		"followup":            "建议先确定组学类型和样本层级，再区分原始数据入口、整理后矩阵和项目级元数据分别检索。",
	}), nil
}

func (t *ScientificDBClinicTool) sequenceRecordRouter(params map[string]interface{}) (string, error) {
	accession, _ := params["accession"].(string)
	gene, _ := params["gene"].(string)
	organism, _ := params["organism"].(string)
	recordType, _ := params["record_type"].(string)
	databaseScope, _ := params["database_scope"].(string)
	keywords := parseScientificDBList(params["keywords"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{accession, gene, organism, recordType, databaseScope}, keywords...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one sequence record search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{accession, gene, organism, recordType}, keywords...)), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	sources := []map[string]interface{}{
		{"source": "NCBI Nucleotide", "url": "https://www.ncbi.nlm.nih.gov/nuccore/?term=" + url.QueryEscape(query), "focus": "核酸记录与版本信息"},
		{"source": "NCBI Gene", "url": "https://www.ncbi.nlm.nih.gov/gene/?term=" + url.QueryEscape(query), "focus": "基因条目与转录本摘要"},
		{"source": "Ensembl", "url": "https://www.ensembl.org/Multi/Search/Results?q=" + url.QueryEscape(query), "focus": "基因组、转录本与注释版本"},
		{"source": "UCSC Genome Browser", "url": "https://genome.ucsc.edu/cgi-bin/hgTracks?db=hg38&position=" + url.QueryEscape(query), "focus": "基因组定位与浏览"},
	}
	if containsAnyNormalizedScientificDB([]string{recordType, databaseScope}, "protein", "peptide", "蛋白") {
		sources = append(sources,
			map[string]interface{}{"source": "NCBI Protein", "url": "https://www.ncbi.nlm.nih.gov/protein/?term=" + url.QueryEscape(query), "focus": "蛋白 accession 与参考序列"},
			map[string]interface{}{"source": "UniProt", "url": "https://www.uniprot.org/uniprotkb?query=" + url.QueryEscape(query), "focus": "蛋白功能、同工型与注释"},
		)
	} else {
		sources = append(sources, map[string]interface{}{"source": "RefSeq", "url": "https://www.ncbi.nlm.nih.gov/refseq/?term=" + url.QueryEscape(query), "focus": "参考序列与 curated 记录"})
	}
	if strings.TrimSpace(accession) != "" {
		sources = append(sources, map[string]interface{}{"source": "European Nucleotide Archive", "url": "https://www.ebi.ac.uk/ena/browser/search?query=" + url.QueryEscape(strings.TrimSpace(accession)), "focus": "按 accession 追溯原始记录与提交信息"})
	}

	return jsonStr(map[string]interface{}{
		"panel":               "sequence_record_router",
		"query":               query,
		"accession":           strings.TrimSpace(accession),
		"gene":                strings.TrimSpace(gene),
		"organism":            strings.TrimSpace(organism),
		"record_type":         strings.TrimSpace(recordType),
		"database_scope":      strings.TrimSpace(databaseScope),
		"keywords":            keywords,
		"recommended_sources": sources,
		"followup":            "建议先确认 accession/基因名和记录类型，再区分 reference record、原始提交记录与浏览器定位入口。",
	}), nil
}

func (t *ScientificDBClinicTool) domainFamilyRouter(params map[string]interface{}) (string, error) {
	gene, _ := params["gene"].(string)
	protein, _ := params["protein"].(string)
	domain, _ := params["domain"].(string)
	family, _ := params["family"].(string)
	organism, _ := params["organism"].(string)
	keywords := parseScientificDBList(params["keywords"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{gene, protein, domain, family, organism}, keywords...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one domain or family search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{gene, protein, domain, family}, keywords...)), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	sources := []map[string]interface{}{
		{"source": "InterPro", "url": "https://www.ebi.ac.uk/interpro/search/text/" + url.QueryEscape(query), "focus": "整合家族、结构域与功能位点"},
		{"source": "Pfam", "url": "https://www.ebi.ac.uk/interpro/search/pfam/?query=" + url.QueryEscape(query), "focus": "蛋白家族与 HMM 模型"},
		{"source": "NCBI CDD", "url": "https://www.ncbi.nlm.nih.gov/Structure/cdd/wrpsb.cgi?seqinput=" + url.QueryEscape(query), "focus": "保守结构域与功能注释"},
		{"source": "SMART", "url": "http://smart.embl.de/smart/set_mode.cgi?NORMAL=1&TEXTONLY=1&ACC=" + url.QueryEscape(query), "focus": "信号肽、结构域架构与模块"},
		{"source": "PROSITE", "url": "https://prosite.expasy.org/cgi-bin/prosite/search-ac?query=" + url.QueryEscape(query), "focus": "保守 motif 与 signature 模式"},
	}
	if strings.TrimSpace(protein) != "" || strings.TrimSpace(gene) != "" {
		sources = append(sources, map[string]interface{}{"source": "UniProt", "url": "https://www.uniprot.org/uniprotkb?query=" + url.QueryEscape(query), "focus": "蛋白功能条目与结构域注释整合"})
	}

	return jsonStr(map[string]interface{}{
		"panel":               "domain_family_router",
		"query":               query,
		"gene":                strings.TrimSpace(gene),
		"protein":             strings.TrimSpace(protein),
		"domain":              strings.TrimSpace(domain),
		"family":              strings.TrimSpace(family),
		"organism":            strings.TrimSpace(organism),
		"keywords":            keywords,
		"recommended_sources": sources,
		"followup":            "建议先统一蛋白名、结构域名或家族名，再交叉核对 InterPro、Pfam、CDD 与 UniProt 的注释一致性。",
	}), nil
}

func (t *ScientificDBClinicTool) transcriptRouter(params map[string]interface{}) (string, error) {
	gene, _ := params["gene"].(string)
	transcriptID, _ := params["transcript_id"].(string)
	accession, _ := params["accession"].(string)
	organism, _ := params["organism"].(string)
	exonHint, _ := params["exon_hint"].(string)
	keywords := parseScientificDBList(params["keywords"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{gene, transcriptID, accession, organism, exonHint}, keywords...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one transcript search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{gene, transcriptID, accession, exonHint}, keywords...)), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	sources := []map[string]interface{}{
		{"source": "NCBI Gene", "url": "https://www.ncbi.nlm.nih.gov/gene/?term=" + url.QueryEscape(query), "focus": "基因条目与转录本摘要"},
		{"source": "RefSeq", "url": "https://www.ncbi.nlm.nih.gov/refseq/?term=" + url.QueryEscape(query), "focus": "参考转录本与 curated accession"},
		{"source": "Ensembl Transcript", "url": "https://www.ensembl.org/Multi/Search/Results?q=" + url.QueryEscape(query), "focus": "转录本、外显子和异构体注释"},
		{"source": "UCSC Genome Browser", "url": "https://genome.ucsc.edu/cgi-bin/hgTracks?db=hg38&position=" + url.QueryEscape(query), "focus": "坐标、外显子结构与浏览器轨道"},
	}
	if strings.TrimSpace(accession) != "" || strings.TrimSpace(transcriptID) != "" {
		sources = append(sources, map[string]interface{}{"source": "European Nucleotide Archive", "url": "https://www.ebi.ac.uk/ena/browser/search?query=" + url.QueryEscape(strings.TrimSpace(transcriptID)+" "+strings.TrimSpace(accession)), "focus": "转录本 accession 与原始记录追溯"})
	}

	return jsonStr(map[string]interface{}{
		"panel":               "transcript_router",
		"query":               query,
		"gene":                strings.TrimSpace(gene),
		"transcript_id":       strings.TrimSpace(transcriptID),
		"accession":           strings.TrimSpace(accession),
		"organism":            strings.TrimSpace(organism),
		"exon_hint":           strings.TrimSpace(exonHint),
		"keywords":            keywords,
		"recommended_sources": sources,
		"followup":            "建议先统一 gene/transcript accession，再区分 curated transcript、浏览器坐标和异构体注释入口。",
	}), nil
}

func (t *ScientificDBClinicTool) genomeBrowserRouter(params map[string]interface{}) (string, error) {
	gene, _ := params["gene"].(string)
	region, _ := params["region"].(string)
	organism, _ := params["organism"].(string)
	genomeBuild, _ := params["genome_build"].(string)
	trackFocus, _ := params["track_focus"].(string)
	keywords := parseScientificDBList(params["keywords"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{gene, region, organism, genomeBuild, trackFocus}, keywords...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one genome browser search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{gene, region, genomeBuild, trackFocus}, keywords...)), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	browserRegion := strings.TrimSpace(region)
	if browserRegion == "" {
		browserRegion = strings.TrimSpace(gene)
	}

	sources := []map[string]interface{}{
		{"source": "UCSC Genome Browser", "url": "https://genome.ucsc.edu/cgi-bin/hgTracks?db=hg38&position=" + url.QueryEscape(browserRegion), "focus": "基因组坐标、变异、表达和调控轨道"},
		{"source": "Ensembl Genome Browser", "url": "https://www.ensembl.org/Multi/Search/Results?q=" + url.QueryEscape(query), "focus": "基因组浏览、注释与 comparative tracks"},
		{"source": "NCBI Genome Data Viewer", "url": "https://www.ncbi.nlm.nih.gov/genome/gdv/browser/?q=" + url.QueryEscape(browserRegion), "focus": "NCBI 基因组浏览和特征图层"},
	}
	if containsAnyNormalizedScientificDB([]string{trackFocus}, "regulation", "chip", "atac", "enhancer", "调控", "表观") {
		sources = append(sources, map[string]interface{}{"source": "ENCODE Browser", "url": "https://www.encodeproject.org/search/?searchTerm=" + url.QueryEscape(query), "focus": "调控组学和功能元件轨道"})
	}
	if containsAnyNormalizedScientificDB([]string{organism}, "human", "homo sapiens", "人") {
		sources = append(sources, map[string]interface{}{"source": "GTEx Portal", "url": "https://gtexportal.org/home/gene/" + url.QueryEscape(strings.TrimSpace(gene)), "focus": "基因表达和 eQTL 关联浏览"})
	}

	return jsonStr(map[string]interface{}{
		"panel":               "genome_browser_router",
		"query":               query,
		"gene":                strings.TrimSpace(gene),
		"region":              strings.TrimSpace(region),
		"organism":            strings.TrimSpace(organism),
		"genome_build":        strings.TrimSpace(genomeBuild),
		"track_focus":         strings.TrimSpace(trackFocus),
		"keywords":            keywords,
		"recommended_sources": sources,
		"followup":            "建议先确认坐标系和 genome build，再决定走 UCSC、Ensembl 还是 NCBI 浏览器查看对应轨道。",
	}), nil
}

func (t *ScientificDBClinicTool) orthologRouter(params map[string]interface{}) (string, error) {
	gene, _ := params["gene"].(string)
	organism, _ := params["organism"].(string)
	targetOrganism, _ := params["target_organism"].(string)
	orthologScope, _ := params["ortholog_scope"].(string)
	keywords := parseScientificDBList(params["keywords"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{gene, organism, targetOrganism, orthologScope}, keywords...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one ortholog search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{gene, organism, targetOrganism}, keywords...)), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	sources := []map[string]interface{}{
		{"source": "Ensembl Compara", "url": "https://www.ensembl.org/Multi/Search/Results?q=" + url.QueryEscape(query), "focus": "跨物种 ortholog/paralog 与基因树"},
		{"source": "NCBI Gene", "url": "https://www.ncbi.nlm.nih.gov/gene/?term=" + url.QueryEscape(query), "focus": "Gene 条目中的 ortholog 与跨物种链接"},
		{"source": "OMA Browser", "url": "https://omabrowser.org/oma/search/?query=" + url.QueryEscape(query), "focus": "保守同源群与系统发育关系"},
		{"source": "OrthoDB", "url": "https://www.orthodb.org/?query=" + url.QueryEscape(query), "focus": "正交群与物种覆盖范围"},
	}
	if containsAnyNormalizedScientificDB([]string{targetOrganism, organism}, "human", "mouse", "rat", "zebrafish", "fly", "worm", "yeast") {
		sources = append(sources, map[string]interface{}{"source": "Alliance Genome", "url": "https://www.alliancegenome.org/search?category=gene&query=" + url.QueryEscape(query), "focus": "模式生物 ortholog 和功能汇总"})
	}

	return jsonStr(map[string]interface{}{
		"panel":               "ortholog_router",
		"query":               query,
		"gene":                strings.TrimSpace(gene),
		"organism":            strings.TrimSpace(organism),
		"target_organism":     strings.TrimSpace(targetOrganism),
		"ortholog_scope":      strings.TrimSpace(orthologScope),
		"keywords":            keywords,
		"recommended_sources": sources,
		"followup":            "建议先统一 gene symbol 和物种名，再区分 ortholog、paralog 与基因家族扩张结果。",
	}), nil
}

func (t *ScientificDBClinicTool) proteinInteractionRouter(params map[string]interface{}) (string, error) {
	gene, _ := params["gene"].(string)
	protein, _ := params["protein"].(string)
	organism, _ := params["organism"].(string)
	interactionType, _ := params["interaction_type"].(string)
	disease, _ := params["disease"].(string)
	keywords := parseScientificDBList(params["keywords"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{gene, protein, organism, interactionType, disease}, keywords...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one protein interaction search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{gene, protein, disease, interactionType}, keywords...)), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	sources := []map[string]interface{}{
		{"source": "STRING", "url": "https://string-db.org/cgi/search?search=" + url.QueryEscape(query), "focus": "PPI 网络、证据通道与功能富集"},
		{"source": "BioGRID", "url": "https://thebiogrid.org/search.php?search=" + url.QueryEscape(query), "focus": "实验支持的物理和遗传互作"},
		{"source": "IntAct", "url": "https://www.ebi.ac.uk/intact/search?query=" + url.QueryEscape(query), "focus": "分子互作证据与实验细节"},
		{"source": "UniProt", "url": "https://www.uniprot.org/uniprotkb?query=" + url.QueryEscape(query), "focus": "蛋白条目中的互作注释与交叉链接"},
	}
	if containsAnyNormalizedScientificDB([]string{interactionType}, "signaling", "signal", "pathway", "调控", "信号") {
		sources = append(sources, map[string]interface{}{"source": "SIGNOR", "url": "https://signor.uniroma2.it/search.php?search=" + url.QueryEscape(query), "focus": "有方向性的信号调控互作"})
	}
	if strings.TrimSpace(disease) != "" {
		sources = append(sources, map[string]interface{}{"source": "DisGeNET", "url": "https://www.disgenet.org/search/0/" + url.QueryEscape(strings.TrimSpace(gene)+" "+strings.TrimSpace(disease)), "focus": "疾病背景下的基因关联补充"})
	}

	return jsonStr(map[string]interface{}{
		"panel":               "protein_interaction_router",
		"query":               query,
		"gene":                strings.TrimSpace(gene),
		"protein":             strings.TrimSpace(protein),
		"organism":            strings.TrimSpace(organism),
		"interaction_type":    strings.TrimSpace(interactionType),
		"disease":             strings.TrimSpace(disease),
		"keywords":            keywords,
		"recommended_sources": sources,
		"followup":            "建议先确认互作对象命名和物种背景，再区分预测网络、实验互作和方向性调控数据库。",
	}), nil
}

func (t *ScientificDBClinicTool) phenotypeRouter(params map[string]interface{}) (string, error) {
	phenotype, _ := params["phenotype"].(string)
	gene, _ := params["gene"].(string)
	disease, _ := params["disease"].(string)
	organism, _ := params["organism"].(string)
	keywords := parseScientificDBList(params["keywords"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{phenotype, gene, disease, organism}, keywords...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one phenotype search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{phenotype, gene, disease}, keywords...)), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	sources := []map[string]interface{}{
		{"source": "Human Phenotype Ontology", "url": "https://hpo.jax.org/app/search?q=" + url.QueryEscape(query), "focus": "标准化 phenotype 术语与层级"},
		{"source": "Monarch Initiative", "url": "https://monarchinitiative.org/search/" + url.QueryEscape(query), "focus": "表型、疾病、基因的整合关系"},
		{"source": "OMIM", "url": "https://omim.org/search?index=entry&search=" + url.QueryEscape(query), "focus": "表型系列与遗传病条目"},
		{"source": "Orphanet", "url": "https://www.orpha.net/en/disease/search?query=" + url.QueryEscape(query), "focus": "罕见病与表型描述"},
	}
	if strings.TrimSpace(gene) != "" {
		sources = append(sources, map[string]interface{}{"source": "ClinVar", "url": "https://www.ncbi.nlm.nih.gov/clinvar/?term=" + url.QueryEscape(strings.TrimSpace(gene)+" "+strings.TrimSpace(phenotype)), "focus": "变异与 phenotype 关联补充"})
	}

	return jsonStr(map[string]interface{}{
		"panel":               "phenotype_router",
		"query":               query,
		"phenotype":           strings.TrimSpace(phenotype),
		"gene":                strings.TrimSpace(gene),
		"disease":             strings.TrimSpace(disease),
		"organism":            strings.TrimSpace(organism),
		"keywords":            keywords,
		"recommended_sources": sources,
		"followup":            "建议先统一 phenotype 术语或 HPO 表述，再交叉核对表型、疾病和基因的映射关系。",
	}), nil
}

func (t *ScientificDBClinicTool) modelOrganismRouter(params map[string]interface{}) (string, error) {
	gene, _ := params["gene"].(string)
	disease, _ := params["disease"].(string)
	phenotype, _ := params["phenotype"].(string)
	modelOrganism, _ := params["model_organism"].(string)
	keywords := parseScientificDBList(params["keywords"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{gene, disease, phenotype, modelOrganism}, keywords...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one model organism search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{gene, disease, phenotype, modelOrganism}, keywords...)), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	sources := []map[string]interface{}{
		{"source": "MGI", "url": "https://www.informatics.jax.org/quicksearch/summary?query=" + url.QueryEscape(query), "focus": "小鼠模型、表型和疾病映射"},
		{"source": "ZFIN", "url": "https://zfin.org/search?category=&q=" + url.QueryEscape(query), "focus": "斑马鱼基因、品系和表型模型"},
		{"source": "WormBase", "url": "https://wormbase.org/search/all/" + url.QueryEscape(query), "focus": "线虫基因和 phenotype 模型"},
		{"source": "FlyBase", "url": "https://flybase.org/search/all/" + url.QueryEscape(query), "focus": "果蝇基因和遗传模型"},
	}
	if containsAnyNormalizedScientificDB([]string{modelOrganism}, "rat", "大鼠") {
		sources = append(sources, map[string]interface{}{"source": "RGD", "url": "https://rgd.mcw.edu/rgdweb/search/search.html?term=" + url.QueryEscape(query), "focus": "大鼠基因、疾病和表型注释"})
	}
	if strings.TrimSpace(gene) != "" || strings.TrimSpace(disease) != "" {
		sources = append(sources, map[string]interface{}{"source": "Alliance Genome", "url": "https://www.alliancegenome.org/search?category=gene&query=" + url.QueryEscape(query), "focus": "跨模式生物基因与疾病模型汇总"})
	}

	return jsonStr(map[string]interface{}{
		"panel":               "model_organism_router",
		"query":               query,
		"gene":                strings.TrimSpace(gene),
		"disease":             strings.TrimSpace(disease),
		"phenotype":           strings.TrimSpace(phenotype),
		"model_organism":      strings.TrimSpace(modelOrganism),
		"keywords":            keywords,
		"recommended_sources": sources,
		"followup":            "建议先明确疾病或表型问题，再选择合适模式生物数据库查看基因、品系和实验模型条目。",
	}), nil
}

func (t *ScientificDBClinicTool) cellLineRouter(params map[string]interface{}) (string, error) {
	cellLine, _ := params["cell_line"].(string)
	disease, _ := params["disease"].(string)
	tissue, _ := params["tissue"].(string)
	organism, _ := params["organism"].(string)
	keywords := parseScientificDBList(params["keywords"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{cellLine, disease, tissue, organism}, keywords...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one cell line search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{cellLine, disease, tissue}, keywords...)), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	sources := []map[string]interface{}{
		{"source": "Cellosaurus", "url": "https://www.cellosaurus.org/search?query=" + url.QueryEscape(query), "focus": "细胞系命名、别名和污染信息"},
		{"source": "ATCC", "url": "https://www.atcc.org/search#q=" + url.QueryEscape(query), "focus": "标准细胞系与培养资源入口"},
		{"source": "DSMZ", "url": "https://www.dsmz.de/collection/catalogue/search?tx_dsmzresources_pi5%5Bsearch%5D=" + url.QueryEscape(query), "focus": "细胞系和生物资源中心目录"},
		{"source": "DepMap", "url": "https://depmap.org/portal/search/" + url.QueryEscape(query), "focus": "肿瘤细胞系依赖性与多组学背景"},
	}
	if containsAnyNormalizedScientificDB([]string{disease}, "cancer", "tumor", "oncology", "肿瘤", "癌") {
		sources = append(sources, map[string]interface{}{"source": "CCLE", "url": "https://portals.broadinstitute.org/ccle/search?q=" + url.QueryEscape(query), "focus": "肿瘤细胞系基因组与表达背景"})
	}

	return jsonStr(map[string]interface{}{
		"panel":               "cell_line_router",
		"query":               query,
		"cell_line":           strings.TrimSpace(cellLine),
		"disease":             strings.TrimSpace(disease),
		"tissue":              strings.TrimSpace(tissue),
		"organism":            strings.TrimSpace(organism),
		"keywords":            keywords,
		"recommended_sources": sources,
		"followup":            "建议先统一 cell line 名称和别名，再核对来源组织、疾病背景与是否存在污染或错配记录。",
	}), nil
}

func (t *ScientificDBClinicTool) epigenomicsRouter(params map[string]interface{}) (string, error) {
	gene, _ := params["gene"].(string)
	region, _ := params["region"].(string)
	tissue, _ := params["tissue"].(string)
	assay, _ := params["assay"].(string)
	organism, _ := params["organism"].(string)
	keywords := parseScientificDBList(params["keywords"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{gene, region, tissue, assay, organism}, keywords...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one epigenomics search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{gene, region, tissue, assay}, keywords...)), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	browserTarget := strings.TrimSpace(region)
	if browserTarget == "" {
		browserTarget = strings.TrimSpace(gene)
	}

	sources := []map[string]interface{}{
		{"source": "ENCODE", "url": "https://www.encodeproject.org/search/?searchTerm=" + url.QueryEscape(query), "focus": "ChIP-seq、ATAC-seq、DNase 和调控轨道"},
		{"source": "Roadmap Epigenomics", "url": "http://www.roadmapepigenomics.org/data/" + url.QueryEscape(query), "focus": "参考表观组学样本与注释"},
		{"source": "GEO", "url": "https://www.ncbi.nlm.nih.gov/gds/?term=" + url.QueryEscape(query), "focus": "公开表观组学数据集与实验记录"},
		{"source": "UCSC Genome Browser", "url": "https://genome.ucsc.edu/cgi-bin/hgTracks?db=hg38&position=" + url.QueryEscape(browserTarget), "focus": "基因组坐标上的表观组学轨道浏览"},
	}
	if containsAnyNormalizedScientificDB([]string{assay}, "methyl", "bisulfite", "甲基化") {
		sources = append(sources, map[string]interface{}{"source": "MethBank", "url": "https://ngdc.cncb.ac.cn/methbank/search?keyword=" + url.QueryEscape(query), "focus": "DNA 甲基化数据与位点资源"})
	}

	return jsonStr(map[string]interface{}{
		"panel":               "epigenomics_router",
		"query":               query,
		"gene":                strings.TrimSpace(gene),
		"region":              strings.TrimSpace(region),
		"tissue":              strings.TrimSpace(tissue),
		"assay":               strings.TrimSpace(assay),
		"organism":            strings.TrimSpace(organism),
		"keywords":            keywords,
		"recommended_sources": sources,
		"followup":            "建议先明确 assay 类型、组织来源和坐标区间，再区分浏览器轨道、项目级数据集和专题资源入口。",
	}), nil
}

func (t *ScientificDBClinicTool) singleCellRouter(params map[string]interface{}) (string, error) {
	gene, _ := params["gene"].(string)
	cellType, _ := params["cell_type"].(string)
	tissue, _ := params["tissue"].(string)
	disease, _ := params["disease"].(string)
	organism, _ := params["organism"].(string)
	keywords := parseScientificDBList(params["keywords"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{gene, cellType, tissue, disease, organism}, keywords...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one single-cell search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{gene, cellType, tissue, disease}, keywords...)), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	sources := []map[string]interface{}{
		{"source": "CELLxGENE", "url": "https://cellxgene.cziscience.com/?q=" + url.QueryEscape(query), "focus": "单细胞表达图谱与细胞群查询"},
		{"source": "Single Cell Expression Atlas", "url": "https://www.ebi.ac.uk/gxa/sc/home?query=" + url.QueryEscape(query), "focus": "单细胞/单核转录组数据集检索"},
		{"source": "Human Cell Atlas", "url": "https://data.humancellatlas.org/explore/projects?filter=" + url.QueryEscape(query), "focus": "人类细胞图谱项目和样本入口"},
		{"source": "PanglaoDB", "url": "https://panglaodb.se/search.html?query=" + url.QueryEscape(query), "focus": "细胞类型 marker 与表达线索"},
	}
	if containsAnyNormalizedScientificDB([]string{disease}, "cancer", "tumor", "oncology", "肿瘤", "癌") {
		sources = append(sources, map[string]interface{}{"source": "TISCH", "url": "http://tisch.comp-genomics.org/search?query=" + url.QueryEscape(query), "focus": "肿瘤微环境单细胞数据和细胞群注释"})
	}

	return jsonStr(map[string]interface{}{
		"panel":               "single_cell_router",
		"query":               query,
		"gene":                strings.TrimSpace(gene),
		"cell_type":           strings.TrimSpace(cellType),
		"tissue":              strings.TrimSpace(tissue),
		"disease":             strings.TrimSpace(disease),
		"organism":            strings.TrimSpace(organism),
		"keywords":            keywords,
		"recommended_sources": sources,
		"followup":            "建议先明确 cell type、组织来源和疾病背景，再区分图谱入口、项目数据集和 marker 资源。",
	}), nil
}

func (t *ScientificDBClinicTool) proteomicsRouter(params map[string]interface{}) (string, error) {
	gene, _ := params["gene"].(string)
	protein, _ := params["protein"].(string)
	tissue, _ := params["tissue"].(string)
	disease, _ := params["disease"].(string)
	assay, _ := params["assay"].(string)
	organism, _ := params["organism"].(string)
	keywords := parseScientificDBList(params["keywords"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{gene, protein, tissue, disease, assay, organism}, keywords...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one proteomics search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{gene, protein, tissue, disease, assay}, keywords...)), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	sources := []map[string]interface{}{
		{"source": "PRIDE", "url": "https://www.ebi.ac.uk/pride/archive/simpleSearch?q=" + url.QueryEscape(query), "focus": "公开蛋白组学项目、质谱实验与结果文件"},
		{"source": "ProteomicsDB", "url": "https://www.proteomicsdb.org/search?input=" + url.QueryEscape(query), "focus": "蛋白表达、肽段和功能背景"},
		{"source": "PeptideAtlas", "url": "https://db.systemsbiology.net/sbeams/cgi/PeptideAtlas/Search?action=GO&search_key=" + url.QueryEscape(query), "focus": "肽段观测证据与蛋白鉴定"},
		{"source": "MassIVE", "url": "https://massive.ucsd.edu/ProteoSAFe/datasets.jsp?query=" + url.QueryEscape(query), "focus": "原始质谱数据和项目检索"},
	}
	if strings.TrimSpace(tissue) != "" || strings.TrimSpace(protein) != "" || strings.TrimSpace(gene) != "" {
		sources = append(sources, map[string]interface{}{"source": "Human Protein Atlas", "url": "https://www.proteinatlas.org/search/" + url.QueryEscape(strings.TrimSpace(firstNonEmptyScientificDB(protein, gene, query))), "focus": "组织表达、亚细胞定位和抗体染色背景"})
	}

	return jsonStr(map[string]interface{}{
		"panel":               "proteomics_router",
		"query":               query,
		"gene":                strings.TrimSpace(gene),
		"protein":             strings.TrimSpace(protein),
		"tissue":              strings.TrimSpace(tissue),
		"disease":             strings.TrimSpace(disease),
		"assay":               strings.TrimSpace(assay),
		"organism":            strings.TrimSpace(organism),
		"keywords":            keywords,
		"recommended_sources": sources,
		"followup":            "建议先明确蛋白/基因对象、组织背景和实验类型，再区分项目级原始数据、肽段证据和组织表达入口。",
	}), nil
}

func (t *ScientificDBClinicTool) metabolomicsRouter(params map[string]interface{}) (string, error) {
	metabolite, _ := params["metabolite"].(string)
	pathway, _ := params["pathway"].(string)
	disease, _ := params["disease"].(string)
	specimen, _ := params["specimen"].(string)
	organism, _ := params["organism"].(string)
	keywords := parseScientificDBList(params["keywords"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{metabolite, pathway, disease, specimen, organism}, keywords...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one metabolomics search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{metabolite, pathway, disease, specimen}, keywords...)), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	sources := []map[string]interface{}{
		{"source": "HMDB", "url": "https://hmdb.ca/unearth/q?query=" + url.QueryEscape(query), "focus": "代谢物注释、谱图和生物来源背景"},
		{"source": "MetaboLights", "url": "https://www.ebi.ac.uk/metabolights/search?query=" + url.QueryEscape(query), "focus": "公开代谢组学项目与样本元数据"},
		{"source": "Metabolomics Workbench", "url": "https://www.metabolomicsworkbench.org/data/index.php?Mode=Search&SearchWord=" + url.QueryEscape(query), "focus": "代谢组学研究、分析方法和实验数据入口"},
		{"source": "MassIVE", "url": "https://massive.ucsd.edu/ProteoSAFe/datasets.jsp?query=" + url.QueryEscape(query), "focus": "原始质谱数据和项目检索"},
	}
	if strings.TrimSpace(pathway) != "" || strings.TrimSpace(metabolite) != "" {
		sources = append(sources, map[string]interface{}{"source": "KEGG", "url": "https://www.kegg.jp/kegg-bin/search_pathway_text?keyword=" + url.QueryEscape(strings.TrimSpace(firstNonEmptyScientificDB(pathway, metabolite, query))), "focus": "代谢通路、化合物和酶关联背景"})
	}

	return jsonStr(map[string]interface{}{
		"panel":               "metabolomics_router",
		"query":               query,
		"metabolite":          strings.TrimSpace(metabolite),
		"pathway":             strings.TrimSpace(pathway),
		"disease":             strings.TrimSpace(disease),
		"specimen":            strings.TrimSpace(specimen),
		"organism":            strings.TrimSpace(organism),
		"keywords":            keywords,
		"recommended_sources": sources,
		"followup":            "建议先明确代谢物对象、样本类型和实验平台，再区分化合物注释、项目级数据和原始质谱入口。",
	}), nil
}

func (t *ScientificDBClinicTool) microbiomeRouter(params map[string]interface{}) (string, error) {
	taxon, _ := params["taxon"].(string)
	bodySite, _ := params["body_site"].(string)
	disease, _ := params["disease"].(string)
	cohort, _ := params["cohort"].(string)
	organism, _ := params["organism"].(string)
	keywords := parseScientificDBList(params["keywords"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{taxon, bodySite, disease, cohort, organism}, keywords...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one microbiome search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{taxon, bodySite, disease, cohort}, keywords...)), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	sources := []map[string]interface{}{
		{"source": "MGnify", "url": "https://www.ebi.ac.uk/metagenomics/search?query=" + url.QueryEscape(query), "focus": "宏基因组/微生物组项目、分析结果和样本背景"},
		{"source": "GMrepo", "url": "https://gmrepo.humangut.info/search?keyword=" + url.QueryEscape(query), "focus": "人群肠道微生物组关联与丰度浏览"},
		{"source": "Human Microbiome Project", "url": "https://portal.hmpdacc.org/", "focus": "人体微生物组项目与样本元数据入口"},
		{"source": "NCBI Taxonomy", "url": "https://www.ncbi.nlm.nih.gov/Taxonomy/Browser/wwwtax.cgi?name=" + url.QueryEscape(strings.TrimSpace(firstNonEmptyScientificDB(taxon, organism, query))), "focus": "菌种命名、分类层级和参考背景"},
	}
	if strings.TrimSpace(disease) != "" {
		sources = append(sources, map[string]interface{}{"source": "Disbiome", "url": "https://www.disbiome.com/", "focus": "疾病与微生物组关联证据浏览"})
	}

	return jsonStr(map[string]interface{}{
		"panel":               "microbiome_router",
		"query":               query,
		"taxon":               strings.TrimSpace(taxon),
		"body_site":           strings.TrimSpace(bodySite),
		"disease":             strings.TrimSpace(disease),
		"cohort":              strings.TrimSpace(cohort),
		"organism":            strings.TrimSpace(organism),
		"keywords":            keywords,
		"recommended_sources": sources,
		"followup":            "建议先明确 body site、目标菌群或疾病场景，再区分项目级宏基因组数据、关联数据库和分类学参考入口。",
	}), nil
}

func (t *ScientificDBClinicTool) pharmacogenomicsRouter(params map[string]interface{}) (string, error) {
	gene, _ := params["gene"].(string)
	drug, _ := params["drug"].(string)
	variant, _ := params["variant"].(string)
	phenotype, _ := params["phenotype"].(string)
	population, _ := params["population"].(string)
	keywords := parseScientificDBList(params["keywords"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{gene, drug, variant, phenotype, population}, keywords...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one pharmacogenomics search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{gene, drug, variant, phenotype}, keywords...)), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	sources := []map[string]interface{}{
		{"source": "PharmGKB", "url": "https://www.pharmgkb.org/search?query=" + url.QueryEscape(query), "focus": "药物-基因-变异-表型关联与临床注释"},
		{"source": "CPIC", "url": "https://cpicpgx.org/guidelines/", "focus": "药物基因组学临床实施指南"},
		{"source": "ClinVar", "url": "https://www.ncbi.nlm.nih.gov/clinvar/?term=" + url.QueryEscape(query), "focus": "变异临床意义与提交记录"},
		{"source": "dbSNP", "url": "https://www.ncbi.nlm.nih.gov/snp/?term=" + url.QueryEscape(strings.TrimSpace(firstNonEmptyScientificDB(variant, query))), "focus": "SNP 标识、位点和参考注释"},
	}
	if strings.TrimSpace(gene) != "" || strings.TrimSpace(drug) != "" {
		sources = append(sources, map[string]interface{}{"source": "PharmVar", "url": "https://www.pharmvar.org/search?query=" + url.QueryEscape(strings.TrimSpace(firstNonEmptyScientificDB(gene, query))), "focus": "药物代谢相关 star allele 与命名背景"})
	}

	return jsonStr(map[string]interface{}{
		"panel":               "pharmacogenomics_router",
		"query":               query,
		"gene":                strings.TrimSpace(gene),
		"drug":                strings.TrimSpace(drug),
		"variant":             strings.TrimSpace(variant),
		"phenotype":           strings.TrimSpace(phenotype),
		"population":          strings.TrimSpace(population),
		"keywords":            keywords,
		"recommended_sources": sources,
		"followup":            "建议先明确药物、基因和变异对象，再区分临床指南、变异证据和 star allele 命名入口。",
	}), nil
}

func (t *ScientificDBClinicTool) immunologyRouter(params map[string]interface{}) (string, error) {
	target, _ := params["target"].(string)
	cellType, _ := params["cell_type"].(string)
	disease, _ := params["disease"].(string)
	assay, _ := params["assay"].(string)
	organism, _ := params["organism"].(string)
	keywords := parseScientificDBList(params["keywords"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{target, cellType, disease, assay, organism}, keywords...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one immunology search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{target, cellType, disease, assay}, keywords...)), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	sources := []map[string]interface{}{
		{"source": "ImmPort", "url": "https://www.immport.org/shared/search?query=" + url.QueryEscape(query), "focus": "免疫学研究项目、队列与样本元数据"},
		{"source": "IEDB", "url": "https://www.iedb.org/result?query=" + url.QueryEscape(query), "focus": "表位、抗原和免疫反应证据"},
		{"source": "ImmGen", "url": "https://www.immgen.org/Resources/", "focus": "免疫细胞表达和分群背景资源"},
		{"source": "Human Protein Atlas", "url": "https://www.proteinatlas.org/search/" + url.QueryEscape(strings.TrimSpace(firstNonEmptyScientificDB(target, cellType, query))), "focus": "免疫相关蛋白、组织表达和单细胞类型线索"},
	}
	if containsAnyNormalizedScientificDB([]string{disease}, "cancer", "tumor", "oncology", "肿瘤", "癌") {
		sources = append(sources, map[string]interface{}{"source": "TISCH", "url": "http://tisch.comp-genomics.org/search?query=" + url.QueryEscape(query), "focus": "肿瘤免疫微环境单细胞数据与免疫细胞注释"})
	}

	return jsonStr(map[string]interface{}{
		"panel":               "immunology_router",
		"query":               query,
		"target":              strings.TrimSpace(target),
		"cell_type":           strings.TrimSpace(cellType),
		"disease":             strings.TrimSpace(disease),
		"assay":               strings.TrimSpace(assay),
		"organism":            strings.TrimSpace(organism),
		"keywords":            keywords,
		"recommended_sources": sources,
		"followup":            "建议先明确靶点、免疫细胞类型和 assay，再区分项目级免疫数据、表位证据和表达资源入口。",
	}), nil
}

func (t *ScientificDBClinicTool) toxicogenomicsRouter(params map[string]interface{}) (string, error) {
	chemical, _ := params["chemical"].(string)
	gene, _ := params["gene"].(string)
	disease, _ := params["disease"].(string)
	exposure, _ := params["exposure"].(string)
	organism, _ := params["organism"].(string)
	keywords := parseScientificDBList(params["keywords"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{chemical, gene, disease, exposure, organism}, keywords...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one toxicogenomics search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{chemical, gene, disease, exposure}, keywords...)), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	sources := []map[string]interface{}{
		{"source": "CTD", "url": "https://ctdbase.org/search/?search=" + url.QueryEscape(query), "focus": "化学物-基因-疾病关联与毒理背景"},
		{"source": "DrugBank", "url": "https://go.drugbank.com/unearth/q?searcher=drugs&query=" + url.QueryEscape(strings.TrimSpace(firstNonEmptyScientificDB(chemical, query))), "focus": "化学物/药物基础信息与靶点背景"},
		{"source": "PubChem", "url": "https://pubchem.ncbi.nlm.nih.gov/#query=" + url.QueryEscape(strings.TrimSpace(firstNonEmptyScientificDB(chemical, query))), "focus": "化合物性质、结构和生物活性背景"},
		{"source": "GEO", "url": "https://www.ncbi.nlm.nih.gov/gds/?term=" + url.QueryEscape(query), "focus": "暴露或毒理转录组数据集入口"},
	}

	return jsonStr(map[string]interface{}{
		"panel":               "toxicogenomics_router",
		"query":               query,
		"chemical":            strings.TrimSpace(chemical),
		"gene":                strings.TrimSpace(gene),
		"disease":             strings.TrimSpace(disease),
		"exposure":            strings.TrimSpace(exposure),
		"organism":            strings.TrimSpace(organism),
		"keywords":            keywords,
		"recommended_sources": sources,
		"followup":            "建议先明确化学物、暴露场景和目标基因，再区分关联证据、化合物背景和组学数据入口。",
	}), nil
}

func (t *ScientificDBClinicTool) biobankRouter(params map[string]interface{}) (string, error) {
	trait, _ := params["trait"].(string)
	cohort, _ := params["cohort"].(string)
	population, _ := params["population"].(string)
	sampleType, _ := params["sample_type"].(string)
	organism, _ := params["organism"].(string)
	keywords := parseScientificDBList(params["keywords"])

	coreTerms := uniqueSortedScientificDBStrings(append([]string{trait, cohort, population, sampleType, organism}, keywords...))
	if len(coreTerms) == 0 {
		return "", fmt.Errorf("at least one biobank search term is required")
	}

	query := strings.Join(uniqueSortedScientificDBStrings(append([]string{trait, cohort, population, sampleType}, keywords...)), " ")
	if query == "" {
		query = strings.Join(coreTerms, " ")
	}

	sources := []map[string]interface{}{
		{"source": "UK Biobank", "url": "https://www.ukbiobank.ac.uk/enable-your-research/search?query=" + url.QueryEscape(query), "focus": "大型人群队列与表型资源入口"},
		{"source": "dbGaP", "url": "https://www.ncbi.nlm.nih.gov/gap/?term=" + url.QueryEscape(query), "focus": "基因型-表型队列与受控数据申请入口"},
		{"source": "European Genome-phenome Archive", "url": "https://ega-archive.org/search?query=" + url.QueryEscape(query), "focus": "受控人群组学与临床研究数据入口"},
		{"source": "BioSamples", "url": "https://www.ebi.ac.uk/biosamples/search/?text=" + url.QueryEscape(query), "focus": "样本类型、来源与元数据检索"},
	}

	return jsonStr(map[string]interface{}{
		"panel":               "biobank_router",
		"query":               query,
		"trait":               strings.TrimSpace(trait),
		"cohort":              strings.TrimSpace(cohort),
		"population":          strings.TrimSpace(population),
		"sample_type":         strings.TrimSpace(sampleType),
		"organism":            strings.TrimSpace(organism),
		"keywords":            keywords,
		"recommended_sources": sources,
		"followup":            "建议先明确 trait、cohort 和样本类型，再区分开放队列入口、受控数据申请与样本元数据资源。",
	}), nil
}

func selectGuidelineSources(specialty, topic, region string) []string {
	sources := []string{"who.int", "nice.org.uk", "cdc.gov"}
	combined := normalizeScientificDBTerm(strings.Join([]string{specialty, topic}, " "))
	switch {
	case strings.Contains(combined, "pediatric") || strings.Contains(combined, "儿科"):
		sources = append(sources, "aap.org")
	case strings.Contains(combined, "asthma") || strings.Contains(combined, "哮喘"):
		sources = append(sources, "ginasthma.org")
	case strings.Contains(combined, "allergy") || strings.Contains(combined, "过敏"):
		sources = append(sources, "eaaci.org", "aaaai.org")
	case strings.Contains(combined, "gastro") || strings.Contains(combined, "腹痛") || strings.Contains(combined, "消化"):
		sources = append(sources, "espghan.org", "naspghan.org")
	case strings.Contains(combined, "endocrine") || strings.Contains(combined, "内分泌"):
		sources = append(sources, "endocrine.org")
	}
	if containsAnyNormalizedScientificDB([]string{region}, "china", "cn", "中国") {
		sources = append(sources, "cma.org.cn")
	}
	return uniqueSortedScientificDBStrings(sources)
}

func firstNonEmptyScientificDB(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func parseScientificDBList(v interface{}) []string {
	items := []string{}
	switch value := v.(type) {
	case []string:
		for _, item := range value {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				items = append(items, trimmed)
			}
		}
	case []interface{}:
		for _, item := range value {
			if trimmed := strings.TrimSpace(fmt.Sprintf("%v", item)); trimmed != "" {
				items = append(items, trimmed)
			}
		}
	case string:
		replacer := strings.NewReplacer("\n", ",", "；", ",", ";", ",", "、", ",", "，", ",")
		for _, part := range strings.Split(replacer.Replace(value), ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				items = append(items, trimmed)
			}
		}
	}
	return uniqueSortedScientificDBStrings(items)
}

func normalizeScientificDBTerm(v string) string {
	replacer := strings.NewReplacer(" ", "", "-", "", "_", "", ".", "", "/", "", "（", "(", "）", ")")
	return strings.ToLower(strings.TrimSpace(replacer.Replace(v)))
}

func uniqueSortedScientificDBStrings(items []string) []string {
	lookup := map[string]string{}
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		key := normalizeScientificDBTerm(trimmed)
		if _, ok := lookup[key]; !ok {
			lookup[key] = trimmed
		}
	}
	result := make([]string, 0, len(lookup))
	for _, value := range lookup {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func extractScientificDBSequence(raw string) string {
	parts := []string{}
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, ">") {
			continue
		}
		for _, r := range trimmed {
			if r >= 'a' && r <= 'z' {
				parts = append(parts, strings.ToUpper(string(r)))
			} else if (r >= 'A' && r <= 'Z') || r == '*' {
				parts = append(parts, string(r))
			}
		}
	}
	return strings.Join(parts, "")
}

func inferScientificDBSequenceType(sequence, hint string) string {
	normalizedHint := normalizeScientificDBTerm(hint)
	if normalizedHint == "dna" || normalizedHint == "rna" || normalizedHint == "protein" {
		return normalizedHint
	}
	letters := strings.ToUpper(strings.TrimSpace(sequence))
	if letters == "" {
		return "unknown"
	}
	if !containsAnyNormalizedScientificDB([]string{letters}, "e", "f", "i", "l", "p", "q", "z", "*") {
		if strings.Contains(letters, "U") && !strings.Contains(letters, "T") {
			return "rna"
		}
		return "dna"
	}
	if strings.Contains(letters, "U") && !strings.Contains(letters, "T") {
		return "rna"
	}
	return "protein"
}

func minScientificDBInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func containsAnyNormalizedScientificDB(values []string, patterns ...string) bool {
	for _, value := range values {
		normalized := normalizeScientificDBTerm(value)
		for _, pattern := range patterns {
			if strings.Contains(normalized, normalizeScientificDBTerm(pattern)) {
				return true
			}
		}
	}
	return false
}
