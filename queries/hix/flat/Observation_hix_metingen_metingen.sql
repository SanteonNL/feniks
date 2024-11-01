SELECT  
identificatienummer as "Patient.id",
    metingid as "resource_id",
    metingid AS id,
    '' AS parent_id,
    'Observation' AS fhir_path,
    'finaal' AS "status", -- example of code conceptmapping, should be mapped to final
    --'onbekendecode' AS "status", -- example of code conceptmapping with unknown code
    'text' AS "category[0].text",
    'text1' AS "category[1].text",
    'http://terminology.hl7.org/CodeSystem/' AS "category[0].coding[0].system",
    'tommy' AS "category[0].coding[0].code",
    'tommy' AS "category[0].coding[0].display",
    'http://terminology.hl7.org/CodeSystem/observation-nogiets' AS "category[0].coding[1].system",
    'laboratory' AS "category[0].coding[1].code",
    'Laboratory' AS "category[0].coding[1].display",
    'http://terminology.hl7.org/CodeSystem/observation-category' AS "category[1].coding[0].system",
    'laboratoryA' AS "category[1].coding[0].code",
    'LaboratoryA' AS "category[1].coding[0].display",
    4.6 AS "valuequantity.value",
    'http://unitsofmeasure.org' AS "valuequantity.system",
    'mg/dL' AS "valuequantity.unit",
   -- '<' AS "valuequantity.comparator",
    'Patient/' || identificatienummer AS "subject.reference",
    'http://terminology.hl7.org/CodeSystem/observation-category' AS "code.coding[0].system",
    'tyy' AS "code.coding[0].code",
    'Laboratory1' AS "code.coding[0].display",
    4.6 AS "valuequantity.value"
    --metingdatumtijd AS "effectiveDateTime"
FROM 
    observation_raw
--WHERE identificatienummer = :Patient.id
limit 1 ;
