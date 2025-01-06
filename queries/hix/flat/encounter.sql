SELECT  
    identificatienummer AS "Patient.id",
    encounter_id AS "resource_id",
    encounter_id AS id,
    '' AS parent_id,
    'Encounter' AS fhir_path,
    'in-progress' AS "status",  -- Example of code concept mapping, should be mapped accordingly
    'text' AS "type[0].text",
    'text1' AS "type[1].text",
    'http://terminology.hl7.org/CodeSystem/encounter-type' AS "type[0].coding[0].system",
    'hospital-visit' AS "type[0].coding[0].code",
    'Hospital Visit' AS "type[0].coding[0].display",
    'http://terminology.hl7.org/CodeSystem/encounter-type' AS "type[0].coding[1].system",
    'inpatient' AS "type[0].coding[1].code",
    'Inpatient' AS "type[0].coding[1].display",
    'http://terminology.hl7.org/CodeSystem/encounter-status' AS "status.coding[0].system",
    'active' AS "status.coding[0].code",
    'Active' AS "status.coding[0].display",
    'City Clinic' AS "serviceProvider.display",
    'Patient/' || identificatienummer AS "subject.reference",
    encounter_start_time AS "period.start",
    encounter_end_time AS "period.end",
    'http://terminology.hl7.org/CodeSystem/encounter-class' AS "class.coding[0].system",
    'IMP' AS "class.code",
    'inpatient encounter' AS "class.display",
    'ReasonSystem1' AS "reason[0].coding[0].system",
    'R01' AS "reason[0].coding[0].code",
    'Acute Chest Pain' AS "reason[0].coding[0].display"
FROM 
    encounter_raw
--WHERE patient_id = :Patient.id
LIMIT 1;