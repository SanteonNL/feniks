SELECT 
  
    identificatienummer AS "Patient.id", -- "subject.reference"    
    questionnaire_id AS "resource_id",
    questionnaire_id AS id,
    '' AS parent_id,
    'Questionnaire' AS fhir_path,
    code_codesystem AS "code[0].coding[0].system", -- "code.coding[0].system"
    code_code AS "code[0].coding[0].code", -- "code.coding[0].code"
    code_display AS "code[0].coding[0].display", -- "code.coding[0].display"
    status AS "status", -- "status"
    date AS "date", -- "date"
    item_linkId AS "item[0].linkId", -- "item[n].linkId"
    item_text AS "item[0].text", -- "item[n].text"
    item_type AS "item[0].type", -- "item[n].type"
    item_code_codesystem AS "item[0].code[0].system", -- "item[n].code.coding[0].system"
    item_code_code AS "item[0].code[0].code", -- "item[n].code.coding[0].code"
    item_code_display AS "item[0].code[0].display" -- "item[n].code.coding[0].display"
FROM 
    questionnaire_raw
-- WHERE identificatienummer = :Patient.id
LIMIT 1;