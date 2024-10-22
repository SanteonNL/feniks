SELECT  
identificatienummer as "Patient.id",
    metingid as "resource_id",
    metingid AS id,
    '' AS parent_id,
    'Observation' AS fhir_path,
    'final' AS "status",
    'text' AS "category[0].text",
    'text1' AS "category[1].text",
    'http://terminology.hl7.org/CodeSystem/observation-category' AS "category[0].coding[0].system",
    'laboratory' AS "category[0].coding[0].code",
    'Laboratory' AS "category[0].coding[0].display",
    'http://terminology.hl7.org/CodeSystem/observation-category' AS "category[1].coding[0].system",
    'laboratory1' AS "category[1].coding[0].code",
    'Laboratory1' AS "category[1].coding[0].display",
    'http://terminology.hl7.org/CodeSystem/observation-category' AS "category[1].coding[2].system",
    'nogiets' AS "category[1].coding[2].code",
    'nogiets' AS "category[1].coding[2].display",
    4.6 AS "valuequantity.value",
    'http://unitsofmeasure.org' AS "valuequantity.system",
    'mg/dL' AS "valuequantity.unit",
   -- '<' AS "valuequantity.comparator",
    'Patient/' || identificatienummer AS "subject.reference",
    metingdatumtijd AS "effectiveDateTime"
FROM 
    observation_raw
--WHERE identificatienummer = :Patient.id
limit 1 ;
